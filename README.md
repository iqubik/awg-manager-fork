# AWG Manager

> Веб-интерфейс для управления AmneziaWG VPN-туннелями на роутерах Keenetic.
В тестовом режиме добавлена поддержка Sing-box (vless tcp, hysteria, trojan, etc)

> **Disclaimer:** AWG Manager — независимый open-source проект, не аффилированный с [Amnezia.org](https://amnezia.org) и Sing-box [SagerNet](https://github.com/SagerNet/sing-box) и не являющийся их официальным продуктом.Програма находится в стадии вечной BETA версии.

![awgm-showcase](https://raw.githubusercontent.com/hoaxisr/awg-manager/develop/scripts/dev/awgm-showcase.webp)

---

## Возможности

- Управление туннелями AmneziaWG/Sing-box через браузер
- Добавление, удаление и мониторинг peer-ов
- Тест скорости с отображением в реальном времени
- График трафика с периодами 1ч / 3ч / 24ч
- Создание AWG серверов на роутере
- DNS-маршрутизация через туннели с поддержкой системных WireGuard-интерфейсов NDMS и системы правил Sing-box
- Просмотр статуса соединения в реальном времени
- Совместимость с Keenetic c использованием Entware (OPKG)

---

## Требования

- Роутер Keenetic с поддержкой Entware, установленный компонент Wireguard

---

## Установка (стабильная версия)

```sh
opkg update && opkg upgrade
opkg install curl
curl -sL https://raw.githubusercontent.com/hoaxisr/awg-manager/master/scripts/install.sh | sh
```

Если у вас недоступен GitHub или при выполнении скрипта установки он "зависает", можете воспользоваться зеркалом: 
```sh
wget -qO- http://repo.hoaxisr.ru/install.sh | sh
```

После установки веб-интерфейс доступен по адресу роутера и обычно по 2222 порту.

---

## Удаление

```sh
opkg remove awg-manager
rm -rf /opt/etc/awg-manager
```

---

## О проекте

AWG Manager создан как независимый инструмент для управления туннелями AmneziaWG/Sing-box непосредственно на роутере, без CLI.

Проект **не аффилирован с Amnezia.org**, не разрабатывается и не поддерживается командой Amnezia. AmneziaWG используется как транспортный протокол.
Проект **не аффилирован с SagerNet**, не разрабатывается и не поддерживается командой SagerNet. Sing-box используется как транспортный протокол.

---

## Сообщество

Telegram: [@awgmanager](https://t.me/awgmanager)

---

## Поддержать проект

Если проект оказался полезным, можно поддержать разработку донатом:

**USDT (TRC20):** ``

---

## Полезное

Установить и управлять AmneziaWG сервером - https://github.com/bivlked/amneziawg-installer

Другой вариант управления AmneziaWG сервером - https://github.com/pumbaX/awg-multi-script

Документация проекта - https://awgm.hoaxisr.ru/install/
