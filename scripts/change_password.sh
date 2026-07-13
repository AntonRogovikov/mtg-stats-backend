#!/bin/bash
# Смена пароля пользователя приложения mtg_stats.
# По умолчанию — «Антон Роговиков». Для другого игрока: -user "Имя Фамилия".
#
# Пароль вводится скрыто в терминале (эхо выключено) и передаётся утилите
# через stdin — поэтому в пароле можно использовать ЛЮБЫЕ символы (кавычки,
# пробелы, $, !, обратные слэши) без экранирования.
#
# Запуск (обязательно через ssh -t, чтобы был терминал для скрытого ввода):
#   ssh -t root@104.128.132.89 /root/change_password.sh
#   ssh -t root@104.128.132.89 /root/change_password.sh -user "Иван Иванов"
set -euo pipefail

# DSN базы берём из systemd-юнита mtg_stats (нигде отдельно не храним).
line=$(grep 'LOCAL_DSN' /etc/systemd/system/mtg_stats.service | head -1)
dsn=${line#*LOCAL_DSN=}
dsn=${dsn%\"}

printf 'Новый пароль: '
read -rs pass; echo
printf 'Повторите:    '
read -rs pass2; echo
if [ "$pass" != "$pass2" ]; then
  echo 'Пароли не совпадают.' >&2
  exit 1
fi

# Пароль уходит в утилиту через stdin (дословно, без интерпретации шеллом).
printf '%s\n' "$pass" | LOCAL_DSN="$dsn" /root/setpass "$@"
