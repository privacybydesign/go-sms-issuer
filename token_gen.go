package main

type TokenGenerator interface {
	GenerateToken() string
}

type DefaultTokenGenerator struct{}

func (tg *DefaultTokenGenerator) GenerateToken() string {
	return "123456"
}
