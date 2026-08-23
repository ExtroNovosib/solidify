package postgres

type Client struct{}

func NewClient() *Client { return &Client{} }
