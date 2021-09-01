package main

// Config is the master config of ARP. ARP takes this config as input and
type Config struct {
	Serverconfig  Serverconfig    `json:"ServerConfig"`
	Redisservers  []Redisservers  `json:"RedisServers"`
	Originredis   Originredis     `json:"OriginRedis"`
	Originservers []Originservers `json:"OriginServers"`
}

type Configcenter struct {
	URL string `json:"url"`
}
type Serverconfig struct {
	Listen       string       `json:"Listen"`
	Port         int          `json:"Port"`
	Configcenter Configcenter `json:"ConfigCenter"`
}
type Pattern struct {
	Path         string `json:"path"`
	Headermatch  string `json:"headerMatch"`
	Keypattern   string `json:"keyPattern"`
	Returnprefix string `json:"returnPrefix"`
	Returnsuffix string `json:"returnSuffix"`
}
type Redisservers struct {
	IP          string    `json:"ip"`
	Port        int       `json:"port"`
	Auth        string    `json:"auth"`
	Maxidle     int       `json:"MaxIdle"`
	Idletimeout int       `json:"IdleTimeout"`
	Maxactive   int       `json:"MaxActive"`
	Db          int       `json:"db"`
	Pattern     []Pattern `json:"Pattern"`
}
type Originredis struct {
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Auth        string `json:"auth"`
	Maxidle     int    `json:"MaxIdle"`
	Idletimeout int    `json:"IdleTimeout"`
	Maxactive   int    `json:"MaxActive"`
	Db          int    `json:"db"`
	Headermatch string `json:"headerMatch"`
	Keypattern  string `json:"keyPattern"`
}
type Originservers struct {
	Schme       string   `json:"schme"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Headermatch string   `json:"headerMatch"`
	Matchlist   []string `json:"matchList"`
}
