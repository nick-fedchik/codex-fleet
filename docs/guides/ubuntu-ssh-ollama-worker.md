# Підключення чистої Ubuntu як `codex-fleet` worker

Це публічна інструкція для власника нового ноутбука з базовою Ubuntu. Після
виконання master зможе підключатися до нього через SSH під окремим користувачем
і запускати Ollama-задачі.

Це інструкція для поточного SSH fallback. У ній worker owner налаштовує SSH на
своєму ноутбуці, після чого master operator додає адресу worker до локального
реєстру. Цільовий outbound onboarding, де worker owner вказує лише адресу master,
є наступним етапом і ще не входить до поточного SSH MVP.

## Що отримує власник worker

Власнику нового Ubuntu-ноутбука потрібно передати:

1. посилання на цей документ;
2. hostname або IP master для аргументу `MASTER_HOST`;
3. для стандартного репозиторію більше нічого: public key master уже лежить у
   `config/master.pub` і є доступним скрипту.

Для fork, offline-режиму або іншого master public key передається окремо одним
рядком через погоджений канал. Не передавати пароль worker-користувача,
приватний SSH-ключ або файли master.
Окремий worker-користувач має бути створений заздалегідь і повинен мати
локальний `sudo`.

Документ можна відкрити без клонування репозиторію. Скрипт завантажується
окремо з GitHub і запускається від імені вже створеного worker-користувача:

```bash
curl -fsSL https://raw.githubusercontent.com/nick-fedchik/codex-fleet/main/scripts/install-ubuntu-worker.sh \
  -o install-ubuntu-worker.sh
chmod 0755 install-ubuntu-worker.sh
./install-ubuntu-worker.sh MASTER_HOST
```

Якщо репозиторій уже склоновано, замість завантаження можна запустити:

```bash
./scripts/install-ubuntu-worker.sh MASTER_HOST
```

Перед реальною інсталяцією можна виконати перевірку без змін у системі:

```bash
./scripts/install-ubuntu-worker.sh --dry-run MASTER_HOST
```

Повторний звичайний запуск безпечний для вже налаштованого worker: `apt` не
перевстановлює наявні пакети, існуючий worker-ключ зберігається, public key не
дублюється, а systemd-сервіси повторно перевіряються та вмикаються. Файл
`~/.config/codex-fleet/worker.env` є згенерованим і перезаписується актуальними
значеннями.

У режимі без клонування installer завантажує canonical public key із GitHub.
Для іншого ключа можна перед запуском задати `CODEX_FLEET_MASTER_KEY_URL` або
`CODEX_FLEET_MASTER_PUBLIC_KEY`.

Передбачається, що користувач має локальний доступ до ноутбука, права `sudo`,
підключення до тієї самої локальної мережі, що й master, і може виконувати команди
в терміналі. Потрібні Ubuntu Desktop або Ubuntu Server із systemd.

Встановлення Codex на worker не потрібне. Worker запускає лише Ollama, а master
керує підключенням і задачами.

Позначення:

- `WORKER_IP` — IP-адреса нового ноутбука в локальній мережі;
- `MASTER_HOST` — hostname або IP-адреса master для автоматичного інсталятора;
- `MASTER_IP` — IP-адреса master, лише якщо потрібно обмежити UFW;
- `WORKER_USER` — окремий Linux-користувач для SSH-доступу;
- `WORKER_ALIAS` — локальний SSH-аліас на master;
- `MODEL_NAME` — модель Ollama, яку потрібно завантажити.

Поля у верхньому регістрі потрібно замінити власними значеннями; самі назви
`WORKER_IP`, `MASTER_HOST`, `MASTER_IP`, `WORKER_USER`, `WORKER_ALIAS`, `WORKER_HOSTNAME` і
`MODEL_NAME` вводити не потрібно.

## Автоматичний інсталятор

Якщо окремий worker-користувач уже створений і має `sudo`, інсталятор можна
завантажити без Git і запустити від імені цього користувача:

```bash
curl -fsSL https://raw.githubusercontent.com/nick-fedchik/codex-fleet/main/scripts/install-ubuntu-worker.sh \
  -o install-ubuntu-worker.sh
chmod 0755 install-ubuntu-worker.sh
./install-ubuntu-worker.sh MASTER_HOST
```

Інсталятор не запускайте через `sudo` і не запускайте від `root`: він визначає
поточний login, використовує його домашній каталог, створює SSH-ключ worker і
записує `~/.config/codex-fleet/worker.env`. Він перевіряє Ubuntu 24.04+, ставить
`openssh-server`, `openssh-client`, `curl`, сертифікати й Ollama, а також вмикає
відповідні systemd-сервіси. Ollama працює як системний `ollama.service`, тому
не залежить від login-сесії worker-користувача і запускається після reboot.

Модель навмисно не завантажується автоматично. Після інсталятора виконайте:

```bash
ollama pull MODEL_NAME
ollama list
```

Перевірити автозапуск і стан сервісу:

```bash
systemctl is-enabled ollama
systemctl is-active ollama
sudo systemctl status ollama --no-pager
```

Після reboot Ollama буде запущена, але модель може бути ще не завантажена в
RAM/VRAM. Це нормально: перший `worker run` завантажить її, або master може
заздалегідь виконати `worker warmup`.

Поточний SSH fallback все ще потребує, щоб master operator знав адресу worker і
додав його до реєстру. `MASTER_HOST` зберігається в конфігурації для наступного
outbound onboarding.

Installer створює worker-ключ для майбутнього outbound-agent. Для поточного
напрямку `master -> SSH -> worker` використовується canonical public key із
`config/master.pub`. Його приватна відповідна частина залишається тільки на
master. Для custom/offline-режиму installer приймає public key через змінну або
інтерактивний prompt.

## 1. Підготувати worker

Відкрити термінал на новому Ubuntu-ноутбуці й виконати:

```bash
sudo apt update
sudo apt install -y openssh-server curl ca-certificates
```

Створити окремого користувача для fleet. Якщо користувач `WORKER_USER` вже існує,
цей крок пропустити:

```bash
sudo adduser WORKER_USER
```

Запустити SSH і додати його до автозапуску:

```bash
sudo systemctl enable --now ssh
sudo systemctl status ssh --no-pager
```

Дізнатися IP-адресу worker:

```bash
hostname
hostname -I
```

Передати `WORKER_IP` власнику master.

За бажанням одразу задати зрозуміле ім'я вузла:

```bash
sudo hostnamectl set-hostname WORKER_HOSTNAME
```

### Чи потрібно редагувати `/etc/hosts`?

Ні, для базового SSH-сценарію це не потрібно. Master підключається до worker за
IP-адресою або DNS-іменем, указаним у власному `~/.ssh/config`. На worker не
потрібно додавати master до `/etc/hosts`.

Якщо в локальній мережі немає DNS, а на master хочеться використовувати ім'я
замість IP, достатньо додати запис лише на master:

```text
WORKER_IP WORKER_HOSTNAME
```

Редагувати `/etc/hosts` на worker варто лише якщо worker сам має ініціювати
з'єднання з master за іменем (наприклад, у майбутньому для outbound-agent).
Навіть тоді краще використовувати локальний DNS або DHCP reservation; IP-адресу
master можна задати безпосередньо в конфігурації агента.

## 2. Підготувати SSH-ключ на master

Цей ключ створюється на master-користувачі, який запускає `codex-fleet`. Якщо
він уже існує, повторно його не створювати:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_codex_fleet \
  -C codex-fleet-master
```

Приватний файл `~/.ssh/id_ed25519_codex_fleet` залишається тільки на master.
На worker передається лише `~/.ssh/id_ed25519_codex_fleet.pub`.

Показати публічний ключ, щоб передати його власнику worker через приватний
канал зв'язку:

```bash
cat ~/.ssh/id_ed25519_codex_fleet.pub
```

Підійде особисте повідомлення, email або інший погоджений канал. Пароль
worker-користувача передавати не потрібно.

Потрібен файл `~/.ssh/id_ed25519_codex_fleet.pub`. Приватний файл без `.pub`
нікому не передавати.

Якщо master уже має окремий ключ для цього fleet, можна використати його
публічну частину замість створення нової пари.

## 3. Додати ключ на worker

На worker інсталятор попросить вставити один рядок master public key. Якщо
інсталятор уже був запущений без ключа, додати його вручну під поточним
`WORKER_USER`:

```bash
install -d -m 700 ~/.ssh
printf '%s\n' 'restrict MASTER_PUBLIC_KEY' >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

`MASTER_PUBLIC_KEY` має бути одним повним рядком, що починається з `ssh-ed25519`
або іншого дозволеного типу OpenSSH. Не додавати переносів рядка всередині
ключа. Префікс `restrict` вимикає TTY, agent forwarding, X11 forwarding і
port forwarding, але залишає дозволеними команди, які потрібні поточному CLI.

Власник worker повідомляє master operator лише `WORKER_HOST` або `WORKER_IP`,
`WORKER_USER` і, якщо потрібно, SSH-порт. Пароль worker-користувача master
operator не отримує.

Після цього master operator перевіряє безпарольний доступ:

```bash
ssh -o BatchMode=yes -o ConnectTimeout=5 \
  WORKER_USER@WORKER_IP \
  'hostname; id -un; ollama list'
```

Не копіювати приватний ключ на worker і не додавати його до GitHub. Файл
`~/.ssh/id_ed25519_codex_fleet_worker`, який створює installer на worker,
призначений для майбутнього outbound-agent і не замінює master-ключ у цьому
SSH-MVP.

## 4. Встановити Ollama на worker

Знову відкрити SSH-сеанс або локальний термінал worker і виконати офіційний
інсталятор Ollama для Linux:

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

Запустити Ollama як systemd-сервіс:

```bash
sudo systemctl enable --now ollama
sudo systemctl status ollama --no-pager
```

Завантажити потрібну модель:

```bash
ollama pull MODEL_NAME
ollama list
ollama ps
```

На старому ноутбуці починати з невеликої моделі, яка відповідає його RAM/VRAM.
Модель визначається конфігурацією worker і може бути змінена без повторного
налаштування SSH.

## 5. Створити SSH-аліас на master

Додати до `~/.ssh/config` master:

```sshconfig
Host WORKER_ALIAS
    HostName WORKER_IP
    User WORKER_USER
    IdentityFile ~/.ssh/id_ed25519_codex_fleet
    IdentitiesOnly yes
    ConnectTimeout 5
```

Перевірити підключення:

```bash
ssh WORKER_ALIAS 'hostname; id -un; ollama list; ollama ps'
```

Очікувано побачити ім'я ноутбука, `WORKER_USER`, список моделей і поточні запущені
моделі.

## 6. Перевірити виконання prompt

```bash
ssh WORKER_ALIAS \
  'ollama run MODEL_NAME "Відповідай одним коротким реченням: worker online?"'
```

Якщо повернуто відповідь моделі, хост готовий до підключення в `codex-fleet`.

Якщо команда не працює, спочатку перевірити SSH окремо, потім стан Ollama:

```bash
ssh WORKER_ALIAS 'systemctl is-active ssh; systemctl is-active ollama'
ssh WORKER_ALIAS 'curl -fsS http://127.0.0.1:11434/api/tags'
```

## 7. Обмежити SSH доступ master

Якщо на worker активний UFW, дозволити SSH лише з локальної адреси master:

```bash
sudo ufw allow from MASTER_IP to any port 22 proto tcp
```

Після перевірки ключового доступу можна вимкнути парольний вхід, але перед цим
обов'язково перевірити новий SSH-сеанс у другому терміналі:

```bash
sudoedit /etc/ssh/sshd_config.d/codex-fleet.conf
```

Вміст:

```text
PasswordAuthentication no
PubkeyAuthentication yes
AllowUsers WORKER_USER
```

Перевірити конфігурацію та перезапустити SSH:

```bash
sudo sshd -t
sudo systemctl restart ssh
```

## Результат

Після цього worker не потребує Codex, NATS або ручного запуску команд. На master
достатньо стабільного SSH-аліаса `WORKER_ALIAS`; fleet зможе виконувати
discovery (`hostname`, `ollama list`, `ollama ps`) і передавати prompt-задачі.

## Не передавати та не відкривати

- не передавати нікому приватний SSH-ключ master;
- не додавати приватні ключі, паролі або локальні IP-адреси до публічного GitHub;
- не відкривати Ollama-порт `11434` для всієї мережі, якщо master може працювати
  через SSH;
- не надавати користувачу worker доступ до робочих каталогів master.

Офіційні базові команди: [Ubuntu OpenSSH Server](https://ubuntu.com/server/docs/how-to/security/openssh-server/)
та [Ollama Linux](https://docs.ollama.com/linux).
