# Pin each dashboard release to a supported Agent Substrate version

Each dashboard release will declare and test one compatible Agent Substrate version while Substrate offers no pre-v1 API compatibility guarantee. The backend will fail clearly when its expected contract is unavailable rather than carrying compatibility adapters for historical API shapes; portability applies across implementations using the supported version.
