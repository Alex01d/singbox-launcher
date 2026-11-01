# SPEC 175 — AWG в embedded Tailscale, Linux-first

Статус **O**. Связанные документы: core FEATURE019 / SPEC149, общий контракт
`contract/` и Android Tailscale feature. Цель — один профиль с защищёнными
Tailscale/AWG назначениями и независимым VLESS для выбранного интернета.
Linux desktop — первый этап; Windows/macOS требуют отдельной платформенной
проверки, мобильные клиенты используют тот же контракт.

## Публичная поставка

Публикуются только переносимые код, контракт, схемы, синтетические fixtures,
спецификации и инструкции проверки. Пользовательские профили, identity state,
логи, device/deployment evidence, бинарники и локальные настройки не входят в
исходную поставку. Синхронизация актуального develop сохраняет входящие
контрактные изменения. Внешняя подготовка выполняется без изменений живого VPN.

## Контракт

- Optional `tailscale.awg = {version:2|3, profile_id?:string, parameters:object}`;
  типы параметров определены core149 и `contract/schema/tailscale_awg.schema.json`.
  AWG3.1 — параметры семейства version3; label не заменяет проверку профиля.
- Профиль — неделимое JSON-значение. Parser/model/generator/backup сохраняют
  все поля; частичная очистка invalid/unknown параметров запрещена. Ядро
  отклоняет невалидный профиль до запуска.
- Общий `on_core_unsupported.action: fail_build` исполняется до `drop_node` и
  удаления полей. Отсутствующая или неизвестная capability
  `with_tailscale_awg` даёт `tailscale_awg_core_unsupported`, сохраняя last-good.
- Форма поддерживает обычный Tailscale, AWG2 и AWG3, import/edit/export JSON,
  optional auth key и существующий interactive login. Headscale остаётся
  стандартным control plane; совпадающий профиль задаётся на data-plane peers.
- Правка существующей записи сохраняет ID/tag и state_directory. Изменение
  Body синхронизирует authored JSON Origin.Raw, как штатный редактор. Экспорт
  профиля не переносит machine identity.
- Optional status показывает configured/applied mode, profile label/fingerprint,
  applied и безопасную ошибку отдельно от доступности peer/login.

## Защищённые маршруты и DNS

Companion preset `tailscale-protected` задаёт конкретный endpoint, явные CIDR и
private DNS suffixes. Правила сохраняются до login, после logout и при исчезновении
live routes, перед generic private-direct/VLESS. Для нескольких tailnets нужны
отдельные guard rules. Заданные защищённые назначения без endpoint отклоняют
build целиком; guard не удаляется при отсутствующей capability.

`underlay_mode=auto|derp` и существующий detour задают внешний proxy carrier.
Ошибка carrier не включает native fallback. Профиль AWG и carrier хранятся
отдельно; отдельная URI-схема или доставка профиля через Headscale не нужны.

## Совместимость и границы

AWG/underlay/guard payload использует integer `lx_backup=3` с
`requires.min_contract=1.2.0` и `core_build_tags`. Обычный backup остаётся2.
Старые/будущие неподдерживаемые markers и обязательные requirements отклоняются
до мутаций. Существующие record flags и merge identity сохраняются.

Linux classic bridge использует существующий authenticated loopback
StartedService API и один stream/cache; UI не выполняет network/disk I/O.
Изолированный launcher останавливает только принадлежащий ему child PID;
проверки не затрагивают установленный bundle или другие VPN-процессы.

## Приёмка

Именованные проверки покрывают atomic import/requirements, fail-build guards,
синтетический корпус и authenticated bridge. Linux сборка использует `x11`.
Полный suite остаётся CI. UI/TUN, network handoff/sleep, Windows/macOS и другие
платформы получают явные результаты на целевых устройствах перед PR/release.
Пройденный control handshake или DISCO ping не доказывает AWG data plane:
нужны реальные TCP/UDP direct/DERP и сохранение fail-closed при ошибках.
