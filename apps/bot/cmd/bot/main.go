// Command bot is the entrypoint for the kitacatat Telegram finance bot.
// It loads configuration, wires the OCR/AI/store/telegram dependencies, and
// runs the bot until interrupted.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	tele "gopkg.in/telebot.v3"

	"github.com/kitacatat/bot/internal/ai"
	"github.com/kitacatat/bot/internal/config"
	"github.com/kitacatat/bot/internal/ocr"
	"github.com/kitacatat/bot/internal/store"
	"github.com/kitacatat/bot/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	st, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	log.Println("connected to database")

	aiClient, err := ai.New(ctx, ai.Settings{
		Provider: cfg.AIProvider,
		Model:    cfg.AIModel,
		APIKey:   cfg.AIAPIKey,
		BaseURL:  cfg.AIBaseURL,
	})
	if err != nil {
		return err
	}

	log.Printf("AI provider=%q model=%q", cfg.AIProvider, cfg.AIModel)

	ocrEngine := ocr.New(cfg.TessdataPrefix)

	bot, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return err
	}

	telegram.New(aiClient, ocrEngine, st, cfg.DashboardURL).Register(bot)

	// Graceful shutdown on SIGINT/SIGTERM.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		log.Println("shutting down...")
		bot.Stop()
	}()

	log.Println("bot started (access is granted to linked dashboard accounts)")
	bot.Start() // blocks until bot.Stop()
	return nil
}
