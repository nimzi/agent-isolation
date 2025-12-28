## Intentionally unsafe: bake host SSH keys into the image

This repository supports (by request) **baking your host SSH keys into the Docker image** so Git operations from inside the container can authenticate to GitHub over SSH.

### What to put here (on the host)

Copy the SSH material you want baked into the image into this folder, for example:

- `id_ed25519` (private key)
- `id_ed25519.pub` (public key)
- `known_hosts` (optional; Dockerfile will also add GitHub host keys)
- `config` (optional)

Example:

```bash
mkdir -p docker_ssh
cp -a "$HOME/.ssh/id_ed25519" docker_ssh/
cp -a "$HOME/.ssh/id_ed25519.pub" docker_ssh/
cp -a "$HOME/.ssh/known_hosts" docker_ssh/ 2>/dev/null || true
cp -a "$HOME/.ssh/config" docker_ssh/ 2>/dev/null || true
```

### Build/rebuild

Rebuild the image after adding/updating keys:

```bash
./recreate-container.sh
```

### Security warning

Putting private keys into an image is dangerous:

- Anyone with the image (or access to your Docker registry / cache) can extract your keys.
- Keys may persist in build caches and image layers.
- Rotating keys later requires rebuilding and ensuring old images are deleted everywhere.

You asked for this anyway — but treat this image as **highly sensitive**.

