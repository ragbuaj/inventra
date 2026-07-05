# Ansible — provisioning VPS Inventra

Mengkodifikasi setup di `docs/DEPLOYMENT.md`. Host tidak butuh Ansible — tooling
berjalan di container (`tools/Dockerfile`).

## Sekali-pakai
```bash
cp inventory.example.ini inventory.ini            # isi IP VPS
cp group_vars/all/vault.example.yml group_vars/all/vault.yml  # isi rahasia, lalu enkripsi
```

## Verifikasi (tanpa menyentuh server)
```bash
ops/ansible/lint.sh          # syntax-check + ansible-lint
```

## Apply ke VPS (langkah operator — butuh SSH + vault password)
```bash
docker run --rm -it -v "$PWD:/work" -w /work -v ~/.ssh:/root/.ssh:ro \
  inventra-ansible-tools ansible-playbook -i inventory.ini site.yml --ask-vault-pass --check   # dry-run
# hilangkan --check untuk apply sungguhan; jalankan 2x → run kedua changed=0 (idempotence)
```
