# IMPLEMENTATION REPORT — 175

Статус **O**: переносимая инженерная реализация подготовлена для совместной
проверки. Полная desktop/mobile/platform приёмка и release остаются открыты.
Приватные профили, журналы, инфраструктурные сведения и device-specific
acceptance evidence в этот публичный отчёт не включаются.

## Поведение

- Contract1.2.0 добавляет atomic `awg`, `underlay_mode` и fatal capability gate.
  Неизвестный capability не превращает профиль в обычный Tailscale.
- Add/edit поддерживает обычный Tailscale, AWG2/AWG3, JSON profile import/export
  и interactive Headscale login без обязательного auth key.
- Companion preset сохраняет конкретный endpoint, защищённые CIDR и private
  DNS suffixes независимо от состояния login/live routes.
- Body и authored JSON origin синхронизируются при правке; локальная запись и
  machine identity сохраняются. Export содержит профиль, но не machine keys.
- Configured/applied mode и безопасная transport error показаны отдельно от
  peer reachability. Linux classic использует существующий authenticated API.
- Backup marker3 + requirements защищает AWG/underlay/guard payload, marker2
  остаётся форматом обычных файлов. Parse/ImportFile/Import10 сохраняют
  profile, flags, merge identity и protected rules атомарно.

## Проверка

После синхронизации актуального develop, общего contract и authoritative core
protobuf выполнены следующие проверки:

- `go test ./core/backup -run '^TestAtomicTransportBackupBoundary$' -count=1` — PASS.
- `go test ./core/build -run '^TestTailscaleAWGContractAndProtection$' -count=1` — PASS.
- `go test ./core/config -run '^TestContractCorpusBody$/singbox/tailscale_awg_profiles$' -count=1` — PASS.
- `go test -tags x11 ./core -run '^TestClassicTailscaleAuthenticatedLoopback$' -count=1` — PASS.
- `go build -tags x11 ./...` — PASS.

Generated stubs сохраняют AWG transport17, WireGuard session counters6/7 и
входящий memoryRuntime10 вместе со всеми текущими RPC. Полный suite не
запускался локально.
Предшествующие инженерные проверки покрывали atomic backup, protected routes,
bridge, corpus и x11 build, а изолированный transport lab — реальный TCP/UDP
при direct/DERP и отрицательные сценарии. Эти результаты не заменяют CI или
ручную проверку на остальных платформах.

## Открытые gates

- Полный suite в CI и ручная desktop UI/TUN приёмка.
- Windows/macOS: privilege, DNS, underlay/reload, sleep и network-change tests.
- Android/mobile handoff и отдельная реальная iOS Packet Tunnel интеграция.
- Immutable published dependency/artifact pins до release; инженерные local
  workspace builds не объявляются опубликованными production artifacts.
