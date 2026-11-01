package core

import (
	"context"
	"sync/atomic"

	"singbox-launcher/core/services"
)

// LegacyBackend — классический движок: делегирует в ProcessService ровно те
// же вызовы, которые раньше делали package-level обёртки. Поведение старого
// режима не меняется ни на байт: вся логика (диалоги, привилегированный
// запуск, Monitor, авто-перезапуск) остаётся внутри ProcessService.
type LegacyBackend struct {
	ac          *AppController
	tailCache   services.TailscaleStatusCache
	tailCancel  context.CancelFunc
	tailEnabled atomic.Bool
}

// NewLegacyBackend constructs the classic spawn backend.
func NewLegacyBackend(ac *AppController) *LegacyBackend {
	b := &LegacyBackend{ac: ac}
	b.startTailscaleAPI()
	return b
}

// Mode implements CoreBackend.
func (b *LegacyBackend) Mode() BackendMode { return BackendClassic }

// StartVPN implements CoreBackend via ProcessService.Start.
func (b *LegacyBackend) StartVPN(skipRunningCheck ...bool) {
	b.ac.ProcessService.Start(skipRunningCheck...)
}

// StopVPN implements CoreBackend via ProcessService.Stop.
func (b *LegacyBackend) StopVPN() {
	b.ac.ProcessService.Stop()
}

// RestartVPN implements CoreBackend via ProcessService.KillForRestart —
// watcher (Monitor / onPrivilegedScriptExited) поднимет процесс обратно.
func (b *LegacyBackend) RestartVPN() {
	b.ac.ProcessService.KillForRestart()
}

// OnAppExit implements CoreBackend: classic всегда останавливает ядро при
// выходе из лаунчера (как делал GracefulExit → StopSingBoxProcess).
func (b *LegacyBackend) OnAppExit() bool {
	b.ac.ProcessService.Stop()
	return true
}

// Close releases the classic backend's local Tailscale status subscription.
func (b *LegacyBackend) Close() {
	if b.tailCancel != nil {
		b.tailCancel()
	}
}
