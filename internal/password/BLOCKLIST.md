# Password blocklist provenance

`common-passwords.bloom` is a generated Bloom filter over the first 100,000
entries in SecLists'
`Passwords/Common-Credentials/xato-net-10-million-passwords-100000.txt`.

- Source repository: `https://github.com/danielmiessler/SecLists`
- Pinned commit: `d3cbcbfe5120ee735dd783e477836619debdc57c`
- Source SHA-256: `1472aafa2561df5e3293aee252aee3ca660c12b399a283cf808bb01b39be388b`
- SecLists license: MIT
- Bloom false-positive target: `0.000001`

Run `go generate ./internal/password` from `services/identity` to reproduce the
asset. The generator refuses content whose SHA-256 differs from the pinned
source.
