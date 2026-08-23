package quotaguard

import "sync"

type tokenBucket struct{}

func (b *tokenBucket) allow() bool { return true }

type QuotaGuard struct {
	mu sync.Mutex
	primary map[string]*tokenBucket
	secondary map[string]*tokenBucket
	tertiary map[string]*tokenBucket
	quaternary map[string]*tokenBucket
	globalHTTP map[string]*tokenBucket
	globalUDP map[string]*tokenBucket
	userHTTP map[string]*tokenBucket
	userUDP map[string]*tokenBucket
	control map[string]*tokenBucket
	ingress map[string]*tokenBucket
	ws map[string]*tokenBucket
	tcp map[string]*tokenBucket
}

func NewQuotaGuard() *QuotaGuard {
	return &QuotaGuard{
		primary: make(map[string]*tokenBucket),
		secondary: make(map[string]*tokenBucket),
		tertiary: make(map[string]*tokenBucket),
		quaternary: make(map[string]*tokenBucket),
		globalHTTP: make(map[string]*tokenBucket),
		globalUDP: make(map[string]*tokenBucket),
		userHTTP: make(map[string]*tokenBucket),
		userUDP: make(map[string]*tokenBucket),
		control: make(map[string]*tokenBucket),
		ingress: make(map[string]*tokenBucket),
		ws: make(map[string]*tokenBucket),
		tcp: make(map[string]*tokenBucket),
	}
}

func (g *QuotaGuard) AllowPrimary(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.primary[key]
	if b == nil {
		b = &tokenBucket{}
		g.primary[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowSecondary(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.secondary[key]
	if b == nil {
		b = &tokenBucket{}
		g.secondary[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowTertiary(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.tertiary[key]
	if b == nil {
		b = &tokenBucket{}
		g.tertiary[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowQuaternary(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.quaternary[key]
	if b == nil {
		b = &tokenBucket{}
		g.quaternary[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowGlobalHTTP(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.globalHTTP[key]
	if b == nil {
		b = &tokenBucket{}
		g.globalHTTP[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowGlobalUDP(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.globalUDP[key]
	if b == nil {
		b = &tokenBucket{}
		g.globalUDP[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowUserHTTP(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.userHTTP[key]
	if b == nil {
		b = &tokenBucket{}
		g.userHTTP[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowUserUDP(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.userUDP[key]
	if b == nil {
		b = &tokenBucket{}
		g.userUDP[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowControl(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.control[key]
	if b == nil {
		b = &tokenBucket{}
		g.control[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowIngress(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.ingress[key]
	if b == nil {
		b = &tokenBucket{}
		g.ingress[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowWs(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.ws[key]
	if b == nil {
		b = &tokenBucket{}
		g.ws[key] = b
	}
	return b.allow()
}

func (g *QuotaGuard) AllowTcp(key string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	b := g.tcp[key]
	if b == nil {
		b = &tokenBucket{}
		g.tcp[key] = b
	}
	return b.allow()
}