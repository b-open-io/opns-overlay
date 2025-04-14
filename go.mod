module github.com/b-open-io/opns-overlay

go 1.24.2

require (
	github.com/4chain-ag/go-overlay-services v0.0.0-00010101000000-000000000000
	github.com/GorillaPool/go-junglebus v0.2.14
	github.com/b-open-io/overlay v0.0.0-20250409215804-0af5fcd6d003
	github.com/bitcoin-sv/go-paymail v0.23.0
	github.com/bitcoin-sv/go-templates v0.0.0-00010101000000-000000000000
	github.com/bsv-blockchain/go-sdk v1.1.22
	github.com/gofiber/fiber/v2 v2.52.6
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.7.3
	github.com/valyala/fasthttp v1.59.0
)

require (
	github.com/andybalholm/brotli v1.1.1 // indirect
	github.com/bitcoin-sv/go-sdk v1.1.18 // indirect
	github.com/bytedance/sonic v1.12.0 // indirect
	github.com/bytedance/sonic/loader v0.2.0 // indirect
	github.com/centrifugal/centrifuge-go v0.10.2 // indirect
	github.com/centrifugal/protocol v0.10.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.4 // indirect
	github.com/cloudwego/iasm v0.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.5 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/gin-gonic/gin v1.10.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.22.0 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/gookit/goutil v0.6.18 // indirect
	github.com/gookit/gsr v0.1.0 // indirect
	github.com/gookit/slog v0.5.8 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/miekg/dns v1.1.63 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/rs/zerolog v1.33.0 // indirect
	github.com/segmentio/asm v1.2.0 // indirect
	github.com/segmentio/encoding v0.3.6 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.12 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	go.elastic.co/ecszerolog v0.2.0 // indirect
	golang.org/x/arch v0.8.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/exp v0.0.0-20220909182711-5c715a9e8561 // indirect
	golang.org/x/mod v0.19.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/term v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	golang.org/x/tools v0.23.0 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/4chain-ag/go-overlay-services => github.com/4chain-ag/go-overlay-services v0.1.1-0.20250414144451-bc2401837c31

// replace github.com/4chain-ag/go-overlay-services => ../go-overlay-services

replace github.com/bsv-blockchain/go-sdk => github.com/b-open-io/go-sdk v1.1.22-0.20250406003733-6a6b9ac5b847

replace github.com/bitcoin-sv/go-templates => github.com/b-open-io/go-templates v0.0.0-20250409215400-150e0d4266fb

replace github.com/b-open-io/overlay => github.com/b-open-io/overlay v0.0.0-20250412200951-7cb7c7e98c67

// replace github.com/b-open-io/overlay => ../overlay
