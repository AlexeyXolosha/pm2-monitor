package telegram

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Client struct {
	b      *bot.Bot
	chatID string
}

func New(token, chatID string) (*Client, error) {
	b, err := bot.New(token)
	if err != nil {
		return nil, fmt.Errorf("Ошибка инициализации телеграм-бота: %w", err)
	}

	return &Client{
		b:      b,
		chatID: chatID,
	}, nil
}

func (c *Client) SendMessage(text string) error {
	params := &bot.SendMessageParams{
		ChatID:    c.chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}

	_, err := c.b.SendMessage(context.Background(), params)
	return err
}
