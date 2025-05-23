# Changelog

## v0.6.0 (unreleased)

- Add support for QUIC 38 and 39, drop support for QUIC 35 and 36
- Added `quic.Config` options for maximal flow control windows
- Add a `quic.Config` option for QUIC versions
- Add a `quic.Config` option to request truncation of the connection ID from a server
- Add a `quic.Config` option to configure the source address validation
- Add a `quic.Config` option to configure the handshake timeout
- Add a `quic.Config` option to configure the idle timeout
- Add a `quic.Config` option to configure keep-alive
- Rename the STK to Cookie
- Implement `net.Conn`-style deadlines for streams
- Remove the `tls.Config` from the `quic.Config`. The `tls.Config` must now be passed to the `Dial` and `Listen` functions as a separate parameter. See the [Godoc](https://godoc.org/github.com/s-anzie/mp-quic) for details.
- Changed the log level environment variable to only accept strings ("DEBUG", "INFO", "ERROR"), see [the wiki](https://github.com/s-anzie/mp-quic/wiki/Logging) for more details.
- Rename the `h2quic.QuicRoundTripper` to `h2quic.RoundTripper`
- Changed `h2quic.Server.Serve()` to accept a `net.PacketConn`
- Drop support for Go 1.7 and 1.8.
- Various bugfixes
