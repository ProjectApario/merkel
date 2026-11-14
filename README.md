# Merkel Tree

![Unique](https://counthis.app/github?repo=projectapario/writer&label=Unique)
![Impressions](https://counthis.app/github?user=projectapario&label=Impressions)

This is a Merkel Tree written in Go that can be used for many different technical applications.

Project Apario will use the Merkel Tree for **Proof of Inclusion**.

### App

```log
╭─andrei@Andreis-Mac-Studio ~/work/projectapario/merkel ‹master●›
╰─$ go run app/main.go
Hash: 50a504831bd50fee3581d287168a85a8dcdd6aa777ffd0fe35e37290268a0153
  Hash: b30ab174f7459cdd40a3acdf15d0c9444fec2adcfb9d579aa154c084885edd0a
    Hash: 559aead08264d5795d3909718cdd05abd49572e84fe55590eef31a88a08fdffd
    Hash: df7e70e5021544f4834bbee64a9e3789febc4be81470df629cad6ddb03320a5c
  Hash: 26b5aabe804fe5d533c663dea833e8078188376ce5ca2b5c3371d09ef6b0657b
    Hash: 6b23c0d5f35d1b11f9b683f0b0a617355deb11277d91ae091d399c655b87940d
    Hash: 3f39d5c348e5b79d06e842c114e6cc571583bbf44e4b0ebfda1a01ec05745d43
Root Hash: 50a504831bd50fee3581d287168a85a8dcdd6aa777ffd0fe35e37290268a0153
Is valid: true
```

### Package

The package `merkel` has either an `app` (a binary) or a package, called `tree`.
