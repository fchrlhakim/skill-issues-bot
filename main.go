package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go-starter-kit/boot"
	"go-starter-kit/infrastructure/config"
	"go-starter-kit/modules/discordbot"
	"go-starter-kit/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	setup, cleanup, err := boot.MakeHandler(cfg)
	if err != nil {
		log.Fatalf("boot app: %v", err)
	}
	defer cleanup()

	app := router.NewHandlerRouter(setup).RouterWithMiddleware()
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           app,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	go func() {
		setup.Logger.Infof("server listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			setup.Logger.Fatalf("server failed: %v", err)
		}
	}()

	if cfg.Discord.BotToken != "" && cfg.Discord.GuildID != "" {
		bot, err := discordbot.New(cfg.Discord.BotToken, cfg.Discord.GuildID, setup.TicketService, setup.MembershipService, setup.Logger)
		if err != nil {
			setup.Logger.Fatalf("discord bot: %v", err)
		}
		if err := bot.Open(); err != nil {
			setup.Logger.Fatalf("discord login: %v", err)
		}
		defer func() { _ = bot.Close() }()
		setup.Logger.Info("discord gateway started")
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		setup.Logger.Fatalf("server shutdown failed: %v", err)
	}
	setup.Logger.Info("server stopped")
}
