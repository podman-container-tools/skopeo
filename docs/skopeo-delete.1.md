% skopeo-delete(1)

## NAME
skopeo\-delete - Mark the _image-name_ for later deletion by the registry's garbage collector.

## SYNOPSIS
**skopeo delete** [*options*] _image-name_

## DESCRIPTION

Mark _image-name_ for deletion.
The effect of this is registry-specific; many registries don’t support this operation, or don’t allow it in some circumstances / configurations.

**WARNING**: If _image-name_ contains a digest, this affects the referenced manifest, and may delete all tags (within the current repository?) pointing to that manifest.

**WARNING**: If _image-name_ contains a tag (but not a digest), in the current version of Skopeo this resolves the tag into a digest, and then deletes the manifest by digest, as described above (possibly deleting all tags pointing to that manifest, not just the provided tag). This behavior may change in the future.

## Enabling deletion on a distribution registry

The `skopeo delete` command only asks the registry to delete a manifest. Whether that is allowed depends on the **registry server's** configuration, not Skopeo's. It is unrelated to `/etc/containers/storage.conf` or other Skopeo client configuration.

When using the [distribution/distribution](https://github.com/distribution/distribution) registry (often run as the `registry` image or `docker-distribution` package), deletion is disabled by default. If it is disabled, `skopeo delete` typically fails with HTTP 405 and an `UNSUPPORTED` error.

To enable deletion on the registry before using `skopeo delete`:

- Set the registry process environment variable **REGISTRY_STORAGE_DELETE_ENABLED=true**, or
- Set the equivalent in the registry config file:

```
storage:
  delete:
    enabled: true
```

The registry configuration file is commonly found on the registry server at `/etc/docker-distribution/registry/config.yml` or `/etc/docker/registry/config.yml`. Restart the registry after changing the setting.

Other hosted registries, such as cloud vendor registries, have their own delete APIs and permissions; enabling `storage.delete` applies only to a self-hosted distribution registry you administer.

Deleting a manifest marks content for removal but does not immediately free disk space. To reclaim space on a distribution registry server, run the registry garbage collector on the server, for example:

```
/usr/bin/registry garbage-collect /etc/docker-distribution/registry/config.yml
```

If the registry runs in a container:

```
$ docker exec -it registry /usr/bin/registry garbage-collect /etc/docker-distribution/registry/config.yml
```

## OPTIONS

See also [skopeo(1)](skopeo.1.md) for options placed before the subcommand name.

**--authfile** _path_

Path of the primary registry credentials file. On Linux, the default is ${XDG\_RUNTIME\_DIR}/containers/auth.json.
See **containers-auth.json**(5) for more details about the credential search mechanism and defaults on other platforms.

Use `skopeo login` to manage the credentials.

The default value of this option is read from the `REGISTRY\_AUTH\_FILE` environment variable.

**--creds** _username[:password]_

Credentials for accessing the registry.

**--cert-dir** _path_

Use certificates at _path_ (*.crt, *.cert, *.key) to connect to the registry.

**--daemon-host** _host_

Use docker daemon host at _host_ (`docker-daemon:` transport only)

**--help**, **-h**

Print usage statement

**--no-creds**

Access the registry anonymously.

**--registry-token** _token_

Bearer token for accessing the registry.

**--retry-times**

The number of times to retry. By default, no retries are attempted.

**--retry-delay**

Fixed delay between retries. If not set (or set to 0s), retry wait time will be exponentially increased based on the number of failed attempts.

**--shared-blob-dir** _directory_

Directory to use to share blobs across OCI repositories.

**--tls-verify**=_bool_

Require HTTPS and verify certificates when talking to the container registry or daemon. Default to registry.conf setting.

**--username**

The username to access the registry.

**--password**

The password to access the registry.

## EXAMPLES

Mark the image `docker://registry.example.com/example/pause:latest` for deletion from the registry.example.com registry.
The registry must allow deletions; see **Enabling deletion on a distribution registry** above for more details.
```console
$ skopeo delete docker://registry.example.com/example/pause:latest
```


## SEE ALSO
skopeo(1), skopeo-login(1), docker-login(1), containers-auth.json(5)

## AUTHORS

Antonio Murdaca <runcom@redhat.com>, Miloslav Trmac <mitr@redhat.com>, Jhon Honce <jhonce@redhat.com>
