package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.podman.io/image/v5/signature"
	"gopkg.in/check.v1"
)

const (
	gpgBinary = "gpg"
)

func init() {
	check.Suite(&SigningSuite{})
}

type SigningSuite struct {
	fingerprint string
}

func findFingerprint(lineBytes []byte) (string, error) {
	lines := string(lineBytes)
	for _, line := range strings.Split(lines, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 10 && fields[0] == "fpr" {
			return fields[9], nil
		}
	}
	return "", errors.New("No fingerprint found")
}

func (s *SigningSuite) SetUpSuite(c *check.C) {
	_, err := exec.LookPath(skopeoBinary)
	c.Assert(err, check.IsNil)

	gpgHome := c.MkDir()
	os.Setenv("GNUPGHOME", gpgHome)

	runCommandWithInput(c, "Key-Type: RSA\nName-Real: Testing user\n%no-protection\n%commit\n", gpgBinary, "--homedir", gpgHome, "--batch", "--gen-key")

	lines, err := exec.Command(gpgBinary, "--homedir", gpgHome, "--with-colons", "--no-permission-warning", "--fingerprint").Output()
	c.Assert(err, check.IsNil)
	s.fingerprint, err = findFingerprint(lines)
	c.Assert(err, check.IsNil)
}

func (s *SigningSuite) TearDownSuite(c *check.C) {
	os.Unsetenv("GNUPGHOME")
}

func (s *SigningSuite) TestSignVerifySmoke(c *check.C) {
	mech, _, err := signature.NewEphemeralGPGSigningMechanism([]byte{})
	c.Assert(err, check.IsNil)
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil { // FIXME? Test that verification and policy enforcement works, using signatures from fixtures
		c.Skip(fmt.Sprintf("Signing not supported: %v", err))
	}

	manifestPath := "fixtures/image.manifest.json"
	dockerReference := "testing/smoketest"

	sigOutput, err := os.CreateTemp("", "sig")
	c.Assert(err, check.IsNil)
	defer os.Remove(sigOutput.Name())
	assertSkopeoSucceeds(c, "^$", "standalone-sign", "-o", sigOutput.Name(),
		manifestPath, dockerReference, s.fingerprint)

	expected := fmt.Sprintf("^Signature verified, digest %s\n$", TestImageManifestDigest)
	assertSkopeoSucceeds(c, expected, "standalone-verify", manifestPath,
		dockerReference, s.fingerprint, sigOutput.Name())
}
