package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

const (
	MaxLogSize   = 1 * 1024 * 1024  // 1MB por arquivo
	MaxTotalSize = 10 * 1024 * 1024 // 10MB total
)

var (
	hookLogger  *log.Logger
	logFile     *os.File
	logFilePath string
)

// Init inicializa o logger de hooks do Claude Code
func Init() error {
	_, homeDir, err := utils.GetActualUser()
	if err != nil {
		return fmt.Errorf("failed to get user directory: %w", err)
	}

	logDir := filepath.Join(homeDir, ".roamie", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFilePath = filepath.Join(logDir, "claude-hooks.log")

	// Rotaciona se necessário
	if err := rotateIfNeeded(); err != nil {
		return fmt.Errorf("failed to rotate logs: %w", err)
	}

	// Abre arquivo fixo
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logFile = f
	hookLogger = log.New(f, "", log.LstdFlags)
	return nil
}

// rotateIfNeeded rotaciona o arquivo de log se necessário
func rotateIfNeeded() error {
	info, err := os.Stat(logFilePath)
	if os.IsNotExist(err) {
		return nil // Arquivo não existe ainda
	}
	if err != nil {
		return err
	}

	// Verifica tamanho
	if info.Size() < MaxLogSize {
		return nil // Ainda não precisa rotacionar
	}

	// Move arquivo atual para nome com timestamp
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := filepath.Join(
		filepath.Dir(logFilePath),
		fmt.Sprintf("claude-hooks-%s.log", timestamp),
	)

	if err := os.Rename(logFilePath, rotatedPath); err != nil {
		return err
	}

	// Limpa arquivos antigos
	return cleanupOldLogs()
}

// cleanupOldLogs remove logs antigos se total exceder MaxTotalSize
func cleanupOldLogs() error {
	logDir := filepath.Dir(logFilePath)
	files, err := filepath.Glob(filepath.Join(logDir, "claude-hooks-*.log"))
	if err != nil {
		return err
	}

	// Ordena por nome (timestamp no nome já ordena)
	sort.Strings(files)

	// Calcula tamanho total
	var totalSize int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		totalSize += info.Size()
	}

	// Remove os mais antigos até ficar abaixo do limite
	for totalSize > MaxTotalSize && len(files) > 0 {
		oldest := files[0]
		info, err := os.Stat(oldest)
		if err == nil {
			totalSize -= info.Size()
		}
		os.Remove(oldest)
		files = files[1:]
	}

	return nil
}

// Info loga mensagem informativa
func Info(format string, v ...interface{}) {
	if hookLogger != nil {
		hookLogger.Printf("[INFO] "+format, v...)
	}
}

// Warning loga mensagem de aviso
func Warning(format string, v ...interface{}) {
	if hookLogger != nil {
		hookLogger.Printf("[WARNING] "+format, v...)
	}
}

// Error loga mensagem de erro
func Error(format string, v ...interface{}) {
	if hookLogger != nil {
		hookLogger.Printf("[ERROR] "+format, v...)
	}
}

// Close fecha o arquivo de log
func Close() error {
	if logFile != nil {
		return logFile.Close()
	}
	return nil
}
