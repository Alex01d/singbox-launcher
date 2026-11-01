# PLAN 175

## Общая последовательность

1. Синхронизировать develop с сохранением внешних резервных копий изменённых
   файлов; семантически совместить AWG с входящими DNS/MTU/dialer изменениями.
2. Расширить authoritative registry/schema/corpus и contract1.2.0. Сохранить
   опубликованные входящие дополнения и handoff Android. Обновить protobuf
   только из согласованного core: AWG transport, WireGuard counters и текущие
   runtime/status поля должны сосуществовать.
3. Проверять fatal capability до удаления узлов/полей; сохранять atomic profile
   и last-good. Поддержать marker3/requirements на всём Parse→ImportFile→Import10
   пути и discovery; обычный export оставить2.
4. Подключить shared Tailscale/AWG form, optional auth key, safe status и
   статические CIDR/private-DNS guards. Сохранить Body/Origin.Raw/identity при
   редактировании, а machine keys исключить из backup/export.
5. Сохранить отдельные data/log/ports для инженерной проверки. Linux classic
   bridge остаётся authenticated loopback; процессные операции касаются только
   своего child PID. Проверить parser→model→generator→backup/import roundtrip.
6. Выполнить только профильные именованные проверки и x11 build; полный suite
   передать CI, ручную и платформенную приёмку оставить явными открытыми gates.

## Подготовка публичного вклада

До внешней публикации сохранить приватный backup исходной рабочей копии,
интегрировать актуальный upstream и составить явный список важных файлов.
Пользовательские настройки/state/cache, logs, бинарники, локальные пути,
идентификаторы устройств, инфраструктура и частные acceptance receipts
исключаются. Спецификации и примеры описывают переносимое поведение и
синтетические test networks. Существующая история реализации сохраняется в
локальном backup, но не переносится в публикуемый diff.

Канонический contract snapshot фиксируется после merge и перед Android sync.
Generated protobuf берётся из окончательного core snapshot; независимые ручные
правки stubs не допускаются. Проверки выпускают локальный отчёт с фактически
выполненными командами. Commit/push выполняется отдельным координатором только
после просмотра curated diff; PR и release на этом этапе не создаются.

## Межплатформенная проверка

Linux: x11 build, отдельный TUN/proxy bundle, private DNS/CIDR fail-closed,
identity/reload и direct/DERP data plane. Android использует общий контракт и
свою VPN service/socket protection. Windows/macOS проверяют свои privilege,
DNS/underlay/reload и network-change механизмы. iOS требует отдельной реальной
Packet Tunnel интеграции. Список проверок не означает их автоматическое
завершение на непроверенных платформах.
