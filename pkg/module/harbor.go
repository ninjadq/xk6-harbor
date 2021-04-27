package module

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dop251/goja"
	rtclient "github.com/go-openapi/runtime/client"
	"github.com/heww/xk6-harbor/pkg/harbor/client"
	"github.com/heww/xk6-harbor/pkg/util"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/modules"
)

var DefaultRootPath = filepath.Join(os.TempDir(), "harbor")

func init() {
	modules.Register("k6/x/harbor", new(Harbor))

	rootPath := os.Getenv("HARBOR_ROOT")
	if rootPath != "" {
		DefaultRootPath = rootPath
	}

	if err := os.MkdirAll(DefaultRootPath, 0755); err != nil {
		panic(err)
	}
}

var (
	varTrue = true
)

type Option struct {
	Scheme   string // http or https
	Host     string
	Username string
	Password string
	Insecure bool // Allow insecure server connections when using SSL
}

type Harbor struct {
	httpClient  *http.Client
	api         *client.HarborAPI
	option      *Option
	initialized bool
	once        sync.Once
}

func (h *Harbor) Initialize(ctx context.Context, args ...goja.Value) {
	if h.initialized {
		common.Throw(common.GetRuntime(ctx), errors.New("harbor module initialized"))
	}

	h.once.Do(func() {
		opt := &Option{
			Scheme:   getEnv("HARBOR_SCHEME", "https"),
			Host:     getEnv("HARBOR_HOST", ""),
			Username: getEnv("HARBOR_USERNAME", "admin"),
			Password: getEnv("HARBOR_PASSWORD", "Harbor12345"),
			Insecure: false,
		}

		if len(args) > 0 {
			if args[0] != nil && !goja.IsUndefined(args[0]) && !goja.IsNull(args[0]) {
				rt := common.GetRuntime(ctx)

				err := rt.ExportTo(args[0], opt)
				Checkf(ctx, err, "failed to parse the option")
			}
		}

		if opt.Host == "" {
			common.GetRuntime(ctx).Interrupt("harbor host is required in initialization")
			return
		}

		opt.Host = strings.TrimSuffix(opt.Host, "/")

		rawURL := fmt.Sprintf("%s://%s/%s", opt.Scheme, opt.Host, client.DefaultBasePath)
		u, err := url.Parse(rawURL)
		if err != nil {
			common.Throw(common.GetRuntime(ctx), err)
		}

		config := client.Config{URL: u}

		if opt.Username != "" && opt.Password != "" {
			config.AuthInfo = rtclient.BasicAuth(opt.Username, opt.Password)
		}

		if opt.Insecure {
			config.Transport = util.NewInsecureTransport()
		} else {
			config.Transport = util.NewDefaultTransport()
		}

		opt.Scheme = strings.ToLower(opt.Scheme)

		h.api = client.New(config)
		h.option = opt
		h.httpClient = &http.Client{Transport: config.Transport}
		h.initialized = true
	})
}

func (h *Harbor) mustInitialized(ctx context.Context) {
	if !h.initialized {
		common.Throw(common.GetRuntime(ctx), errors.New("harbor module not initialized"))
	}
}
