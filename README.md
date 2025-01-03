# Cart

Shopping cart using local storage + stripe checkout using golang webassembly (compiled with tinygo)

## Requires

* golang
* tinygo

## Running

* Set stripe api keys in `server.conf`

* run the test server:

```
$ MENV=server.conf go run server.go --help
stripe test server for webassembly

Usage:
  srv [flags]

Flags:
  -a, --teststripekey         use stripe test api keys instead of live key env: TESTSTRIPEKEY
 (default true)               
  -b, --stripelivesk string   stripe live api sk env: STRIPELIVESK
 (default "sk_live_...")      
  -c, --stripelivepk string   stripe live api pk env: STRIPELIVEPK
 (default "pk_live_...")      
  -d, --stripetestsk string   stripe test api sk env: STRIPETESTSK
 (default "sk_test_...")      
  -e, --stripetestpk string   stripe test api pk env: STRIPETESTPK
 (default "pk_test_...")      
  -f, --webport int           port to serve on env: WEBPORT
 (default 8080)               
  -h, --help                  help for srv
```



```
[GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
 - using env:	export GIN_MODE=release
 - using code:	gin.SetMode(gin.ReleaseMode)

[GIN-debug] GET    /                         --> main.init.func1.1 (3 handlers)
[GIN-debug] GET    /complete                 --> main.init.func1.2 (3 handlers)
[GIN-debug] GET    /order/:piid              --> main.init.func1.3 (3 handlers)
[GIN-debug] POST   /create-payment-intent    --> main.init.func1.4 (3 handlers)
[GIN-debug] POST   /submit-order             --> main.init.func1.5 (3 handlers)
listening on http://127.0.0.1:8080 using gin router
[GIN-debug] [WARNING] You trusted all proxies, this is NOT safe. We recommend you to set a value.
Please check https://pkg.go.dev/github.com/gin-gonic/gin#readme-don-t-trust-all-proxies for details.
[GIN-debug] Listening and serving HTTP on :8080
2025/01/02 13:32:35 compiling wasm binary with tinygo
bash -c 'GOOS=js GOARCH=wasm tinygo build -target=wasm --no-debug -ldflags="-X 'main.stripePK=pk_test_...'" -o /dev/stdout checkout_wasm.go'
2025/01/02 13:32:47 wasm binary size: 455.80 KB
2025/01/02 13:32:47 compile time: 11.616638554s
[GIN] | 2025/01/02 - 13:33:53 | 200 |    8.626434ms |       127.0.0.1 |                                                          127.0.0.1:47960 | GET      /
2025/01/02 13:34:00 Raw request body: {"items":[{"id":"VT-8AW8A X 1","amount":600},{"id":"VT-12CU5 X 1","amount":300},{"id":"shipping-to|John Q. Public|123 Skidoo Street|Philadelphia|PA|19129|United States|4696854921 X 1","amount":700}]}
2025/01/02 13:34:00 Created PaymentIntent with ClientSecret: pi_...
[GIN] | 2025/01/02 - 13:34:00 | 200 |  429.506471ms |       127.0.0.1 |                                                          127.0.0.1:47960 | POST     /create-payment-intent
[GIN] | 2025/01/02 - 13:34:14 | 200 |   12.473741ms |       127.0.0.1 |                                                          127.0.0.1:47960 | GET      /complete
2025/01/02 13:34:14 Received order data: map[cartItems:[map[amount:600 id:VT-8AW8A quantity:1] map[amount:300 id:VT-12CU5 quantity:1] map[amount:700 id:shipping-to|John Q. Public|123 Skidoo Street|Philadelphia|PA|19129|United States|4696854921 quantity:1]]]
2025/01/02 13:34:14 Received payment intent ID: pi_3Qcu9cCAQwDfFjHh04TXIz1Q
[GIN] | 2025/01/02 - 13:34:14 | 200 |  167.674814ms |       127.0.0.1 |                                                          127.0.0.1:47960 | POST     /submit-order
[GIN] | 2025/01/02 - 13:34:21 | 200 |    3.722628ms |       127.0.0.1 |                                                          127.0.0.1:47960 | GET      /order/pi_3Qcu9cCAQwDfFjHh04TXIz1Q

```
