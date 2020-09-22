
# Smarthost client mode

As of version 1.6 (2020-XX), [chasquid] supports operating as a [smarthost]
client.

In this mode, chasquid will deliver all accepted mail (both local and remote)
to a single specific host (the *smarthost* server).

## Status

It is **EXPERIMENTAL** for now. The configuration options and behaviour can
change in backwards-incompatible ways.


## Security

chasquid will always negotiate TLS on the connection to the smarthost, and
expects a valid certificate.

If TLS is not available, or the certificate is not valid, the mail will remain
in the queue and will not be delivered.


## Configuring

Add the following line to `/etc/chasquid/chasquid.conf`:

```
smarthost_url: "smtp://user:password@server:587"
```

Replace `user` and `password` with the credentials used to authenticate to the
smarthost server, and `server:587` with the server address, including port.

You can also use the `tls` scheme for direct TLS connections (usually on port
465).


[chasquid]: https://blitiri.com.ar/p/chasquid
[smarthost]: https://en.wikipedia.org/wiki/Smart_host
