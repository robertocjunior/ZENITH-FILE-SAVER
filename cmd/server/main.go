package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"zenith-file-saver/internal/config"
	"zenith-file-saver/internal/db"
	"zenith-file-saver/internal/gemini"
	"zenith-file-saver/internal/storage"
	"zenith-file-saver/internal/web"
	"zenith-file-saver/internal/whatsapp"
)

func main() {
	log.Println("[Sistema] Iniciando Zap File Saver...")

	dataDir := "data"
	filesDir := "FILES"

	// 1. Initialize Config Manager
	configMgr, err := config.NewManager(dataDir)
	if err != nil {
		log.Fatalf("[Critico] Falha ao carregar configurações: %v", err)
	}
	cfg := configMgr.Get()

	// 2. Initialize App SQLite Database (for logs)
	database, err := db.NewDB(dataDir)
	if err != nil {
		log.Fatalf("[Critico] Falha ao iniciar banco de dados do app: %v", err)
	}
	defer func() {
		log.Println("[Sistema] Fechando banco de dados do app...")
		database.Close()
	}()

	// 3. Initialize Storage Manager
	storageMgr, err := storage.NewManager(filesDir)
	if err != nil {
		log.Fatalf("[Critico] Falha ao iniciar gerenciador de armazenamento: %v", err)
	}

	// 4. Initialize Gemini Client Wrapper
	geminiClient := gemini.NewClient()

	// 5. Setup circular dependencies using local variables and closures
	var webServer *web.Server

	logCallback := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[WhatsMeow] %s", msg)
		if webServer != nil {
			webServer.BroadcastLog("[WhatsApp] %s", msg)
		}
	}

	qrCallback := func(qrBase64 string) {
		if webServer != nil {
			webServer.BroadcastQR(qrBase64)
		}
	}

	stateCallback := func(state string) {
		log.Printf("[WhatsApp] Novo estado de conexão: %s", state)
		if webServer != nil {
			webServer.BroadcastState(state)
			webServer.BroadcastLog("[WhatsApp] Estado alterado para: %s", state)
		}
	}

	mediaCallback := func(data []byte, mimeType, originalName, senderName, senderJID string, timestamp time.Time) {
		msgReceived := fmt.Sprintf("Arquivo detectado de '%s': '%s' (MIME: %s, bytes: %d)", senderName, originalName, mimeType, len(data))
		log.Printf("[WhatsApp] %s", msgReceived)
		
		if webServer != nil {
			webServer.BroadcastLog("[WhatsApp] %s", msgReceived)
		}

		// Calculate SHA-256 checksum of the downloaded file bytes
		hashBytes := sha256.Sum256(data)
		fileHash := fmt.Sprintf("%x", hashBytes)

		// Verify duplicate: check if this file hash was already successfully saved
		exists, err := database.HashExists(fileHash)
		if err != nil {
			log.Printf("[Erro] Falha ao verificar duplicidade de hash: %v", err)
		}
		if exists {
			dupMsg := fmt.Sprintf("Arquivo duplicado detectado (hash: %s). Ignorando para evitar processamento repetido.", fileHash)
			log.Printf("[WhatsApp] %s", dupMsg)
			if webServer != nil {
				webServer.BroadcastLog("[WhatsApp] %s", dupMsg)
			}
			return
		}

		currentCfg := configMgr.Get()
		apiKey := currentCfg.GeminiAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}

		if apiKey == "" {
			errMsg := "Chave de API do Gemini não está configurada"
			log.Printf("[Erro] %s", errMsg)
			if webServer != nil {
				webServer.BroadcastLog("[Erro] Cancelando processamento: %s", errMsg)
				logRecord, _ := database.LogFile(senderName, senderJID, originalName, "-", "-", "-", fileHash, "failed", errMsg)
				if logRecord != nil {
					webServer.BroadcastFileProcessed(*logRecord)
				}
			}
			return
		}

		if webServer != nil {
			webServer.BroadcastLog("[Gemini] Enviando arquivo para classificação...")
		}

		var result *gemini.ClassificationResult
		var classifyErr error

		// Retry logic: try 3 times total (1 initial + 2 retries)
		for attempt := 1; attempt <= 3; attempt++ {
			if attempt > 1 {
				retryMsg := fmt.Sprintf("[Gemini] Falha na tentativa %d. Tentando novamente (%d de 3) em 2 segundos...", attempt-1, attempt)
				log.Printf("[WhatsApp] %s", retryMsg)
				if webServer != nil {
					webServer.BroadcastLog("%s", retryMsg)
				}
				time.Sleep(2 * time.Second)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			result, classifyErr = geminiClient.ClassifyFile(ctx, apiKey, currentCfg.GeminiModel, data, mimeType)
			cancel()

			if classifyErr == nil {
				break
			}
		}

		// If classification still failed after all retries, save the file to the ERROS directory
		if classifyErr != nil {
			errMsg := fmt.Sprintf("Erro na análise do Gemini após 3 tentativas: %v", classifyErr)
			log.Printf("[Erro] %s", errMsg)

			// Save original file to error directory
			errFilename, errPath, saveErr := storageMgr.SaveErrorFile(data, mimeType, originalName, senderName)
			if saveErr != nil {
				log.Printf("[Erro] Falha ao salvar arquivo na pasta de erros: %v", saveErr)
			} else {
				log.Printf("[Sistema] Arquivo gravado na pasta de erros: %s", errPath)
			}

			if webServer != nil {
				webServer.BroadcastLog("[Erro] %s", errMsg)
				if saveErr == nil {
					webServer.BroadcastLog("[Sistema] Arquivo salvo na pasta de erros: %s", errPath)
				}
				logRecord, _ := database.LogFile(senderName, senderJID, originalName, errFilename, "erro", errPath, fileHash, "failed", errMsg)
				if logRecord != nil {
					webServer.BroadcastFileProcessed(*logRecord)
				}
			}
			return
		}

		if webServer != nil {
			webServer.BroadcastLog("[Gemini] Classificação com sucesso! Categoria: '%s', Slug: '%s', Data: '%s'", result.Category, result.Description, result.Date)
		}

		// Intercept irrelevant/meaningless files
		if result.Category == "irrelevante" {
			if webServer != nil {
				webServer.BroadcastLog("[Gemini] Arquivo detectado como irrelevante/sem sentido. Salvando na pasta de itens irrelevantes...")
			}
			irrelFilename, irrelPath, saveErr := storageMgr.SaveIrrelevantFile(data, mimeType, originalName, senderName, result.Date, result.Description)
			if saveErr != nil {
				log.Printf("[Erro] Falha ao salvar arquivo irrelevante: %v", saveErr)
			} else {
				log.Printf("[Sistema] Arquivo irrelevante salvo: %s", irrelPath)
			}

			if webServer != nil {
				if saveErr == nil {
					webServer.BroadcastLog("[Sistema] Salvo na pasta de irrelevantes: %s", irrelPath)
				}
				logRecord, _ := database.LogFile(senderName, senderJID, originalName, irrelFilename, "irrelevante", irrelPath, fileHash, "success", "")
				if logRecord != nil {
					webServer.BroadcastFileProcessed(*logRecord)
				}
			}
			return
		}

		if webServer != nil {
			webServer.BroadcastLog("[Sistema] Salvando arquivo em disco...")
		}

		newName, savedPath, err := storageMgr.SaveFile(data, mimeType, originalName, senderName, result.Date, result.Description)
		if err != nil {
			errMsg := fmt.Sprintf("Erro ao gravar arquivo na pasta: %v", err)
			log.Printf("[Erro] %s", errMsg)
			if webServer != nil {
				webServer.BroadcastLog("[Erro] %s", errMsg)
				logRecord, _ := database.LogFile(senderName, senderJID, originalName, "-", result.Category, "-", fileHash, "failed", errMsg)
				if logRecord != nil {
					webServer.BroadcastFileProcessed(*logRecord)
				}
			}
			return
		}

		// Log success to SQLite DB
		logRecord, err := database.LogFile(senderName, senderJID, originalName, newName, result.Category, savedPath, fileHash, "success", "")
		if err != nil {
			log.Printf("[Erro] Falha ao registrar log no banco SQLite: %v", err)
			return
		}

		log.Printf("[Sucesso] Arquivo processado: %s -> %s", originalName, savedPath)
		if webServer != nil {
			webServer.BroadcastLog("[Sucesso] Processado e salvo com êxito em '%s'", savedPath)
			webServer.BroadcastFileProcessed(*logRecord)
		}
	}

	// 6. Instantiate WhatsApp Client Manager
	whatsappDBPath := filepath.Join(dataDir, "whatsapp.db")
	waMgr := whatsapp.NewClientManager(whatsappDBPath, logCallback, qrCallback, stateCallback, mediaCallback)
	waMgr.SetMonitoredGroup(cfg.MonitoredGroupJID)

	// 7. Instantiate Web Server
	webServer = web.NewServer(configMgr, waMgr, database)

	// 8. Start WhatsApp service
	go func() {
		if err := waMgr.Start(); err != nil {
			log.Printf("[Erro WhatsApp] Erro crítico ao iniciar whatsmeow: %v", err)
			if webServer != nil {
				webServer.BroadcastLog("[Erro] Falha crítica do WhatsApp: %v", err)
			}
		}
	}()

	// 9. Start HTTP Router
	handler, err := webServer.RegisterRoutes()
	if err != nil {
		log.Fatalf("[Critico] Falha ao registrar rotas do servidor web: %v", err)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	go func() {
		log.Printf("[Sistema] Servidor Web escutando na porta %s...", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Critico] Erro ao iniciar servidor web: %v", err)
		}
	}()

	// 10. Handle OS Signals for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Println("[Sistema] Sinal de interrupção recebido. Iniciando encerramento gracioso...")

	// Create shutdown context
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	// Disconnect WhatsApp
	waMgr.Disconnect()

	// Shutdown Web Server
	if err := srv.Shutdown(ctxShutdown); err != nil {
		log.Printf("[Erro] Erro durante o desligamento do servidor web: %v", err)
	}

	log.Println("[Sistema] Serviço desligado com sucesso.")
}
