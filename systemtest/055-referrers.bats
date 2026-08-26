#!/usr/bin/env bats
#
# Tests for OCI Referrers API signature discovery
#

load helpers

function setup() {
    standard_setup

    # Ensure a v2 registries.conf is used (the system one may be v1)
    REGISTRIES_CONF=$TESTDIR/registries.conf
    cat >$REGISTRIES_CONF <<EOF
unqualified-search-registries = ["docker.io", "quay.io"]
EOF
    export CONTAINERS_REGISTRIES_CONF=$REGISTRIES_CONF

    start_registry reg
}

function teardown() {
    podman rm -f reg

    standard_teardown
}

# Helper: create a registries.d config with use-sigstore-attachments for localhost
function setup_sigstore_registries_d() {
    REGISTRIES_D=$TESTDIR/registries.d
    mkdir -p $REGISTRIES_D
    cat >$REGISTRIES_D/registries.yaml <<EOF
docker:
   localhost:5000:
        use-sigstore-attachments: true
EOF
}

# Helper: create a registries.d config with use-sigstore-attachments for ghcr.io
function setup_ghcr_registries_d() {
    REGISTRIES_D=$TESTDIR/registries.d
    mkdir -p $REGISTRIES_D
    cat >$REGISTRIES_D/registries.yaml <<EOF
docker:
   ghcr.io:
        use-sigstore-attachments: true
   localhost:5000:
        use-sigstore-attachments: true
EOF
}

# Helper: generate a sigstore key pair with an empty passphrase
function generate_sigstore_key() {
    PASSPHRASE_FILE=$TESTDIR/passphrase
    echo -n "" > $PASSPHRASE_FILE
    run_skopeo generate-sigstore-key \
               --output-prefix $TESTDIR/sigstore-key \
               --passphrase-file $PASSPHRASE_FILE
    SIGSTORE_PRIVATE_KEY=$TESTDIR/sigstore-key.private
    SIGSTORE_PUBLIC_KEY=$TESTDIR/sigstore-key.pub
}

@test "referrers: tag schema fallback when registry lacks Referrers API" {
    setup_sigstore_registries_d

    # Push an image to the local registry (registry:2.8.2 lacks Referrers API)
    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/referrers-fallback:latest

    # Pull with debug; the standard registry doesn't support the Referrers API
    # so we expect both the initial attempt and the tag schema fallback
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false \
               docker://localhost:5000/referrers-fallback:latest \
               dir:$TESTDIR/pulled

    expect_output --substring "Looking for OCI referrers"
    expect_output --substring "falling back to tag schema"
}

@test "referrers: disabled when use-sigstore-attachments is not set" {
    # Create a registries.d config WITHOUT use-sigstore-attachments
    REGISTRIES_D=$TESTDIR/registries.d
    mkdir -p $REGISTRIES_D
    cat >$REGISTRIES_D/registries.yaml <<EOF
docker:
   localhost:5000:
        lookaside: ""
EOF

    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/referrers-disabled:latest

    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false \
               docker://localhost:5000/referrers-disabled:latest \
               dir:$TESTDIR/pulled

    expect_output --substring "Not looking for sigstore referrers: disabled by configuration"
}

@test "referrers: sigstore signature round-trip with cosign tag" {
    setup_sigstore_registries_d
    generate_sigstore_key

    # Push an unsigned image to the local registry
    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/roundtrip:latest

    # Copy within the registry, signing with the sigstore key.
    # This exercises both the referrers write path (best-effort, will fail
    # on registry:2.8.2) and the cosign tag write path.
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false --dest-tls-verify=false \
               --sign-by-sigstore-private-key $SIGSTORE_PRIVATE_KEY \
               --sign-passphrase-file $PASSPHRASE_FILE \
               docker://localhost:5000/roundtrip:latest \
               docker://localhost:5000/roundtrip:signed

    # The referrers write was attempted (best-effort)
    expect_output --substring "Looking for OCI referrers"

    # Pull the signed image back and verify both discovery paths run.
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false \
               docker://localhost:5000/roundtrip:signed \
               dir:$TESTDIR/pulled-signed

    # Referrers discovery ran (and fell back to tag schema)
    expect_output --substring "Looking for OCI referrers"
    # The cosign tag (.sig suffix) contains the signature
    expect_output --substring "Found a sigstore attachment manifest"
}

@test "referrers: re-sign preserves existing signatures" {
    setup_sigstore_registries_d
    generate_sigstore_key

    # Push and sign an image
    run_skopeo copy --dest-tls-verify=false \
               docker://quay.io/libpod/busybox:latest \
               docker://localhost:5000/resign:latest

    run_skopeo --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false --dest-tls-verify=false \
               --sign-by-sigstore-private-key $SIGSTORE_PRIVATE_KEY \
               --sign-passphrase-file $PASSPHRASE_FILE \
               docker://localhost:5000/resign:latest \
               docker://localhost:5000/resign:signed

    # Re-sign the same image (adds a second signature to the cosign tag).
    # Both referrers and cosign tag sources are checked; the dedup logic
    # prevents duplicates.
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false --dest-tls-verify=false \
               --sign-by-sigstore-private-key $SIGSTORE_PRIVATE_KEY \
               --sign-passphrase-file $PASSPHRASE_FILE \
               docker://localhost:5000/resign:signed \
               docker://localhost:5000/resign:resigned

    # Both discovery paths must have run
    expect_output --substring "Looking for OCI referrers"
    expect_output --substring "Found a sigstore attachment manifest"

    # Pull the re-signed image and verify it succeeds without errors
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy --src-tls-verify=false \
               docker://localhost:5000/resign:resigned \
               dir:$TESTDIR/pulled-resigned

    expect_output --substring "Found a sigstore attachment manifest"
}

@test "referrers: discover referrers from real ghcr.io image" {
    setup_ghcr_registries_d

    # ghcr.io/saschagrunert/nri-supply-chain has cosign signatures and
    # referrers tag schema entries, providing a real-world test target.
    run_skopeo --debug --registries.d $REGISTRIES_D \
               copy docker://ghcr.io/saschagrunert/nri-supply-chain:latest \
               dir:$TESTDIR/nri-supply-chain

    # Referrers discovery should have been attempted against ghcr.io
    expect_output --substring "Looking for OCI referrers"
}

# vim: filetype=sh
