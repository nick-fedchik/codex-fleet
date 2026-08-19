# Підключення чистої Ubuntu як `codex-fleet` worker

Це публічна інструкція для власника нового ноутбука з базовою Ubuntu. Після
виконання master зможе підключатися до нього через SSH під окремим користувачем
і запускати Ollama-задачі.

Це інструкція для поточного SSH fallback. У ній worker owner налаштовує SSH на
своєму ноутбуці, після чого master operator додає адресу worker до локального
реєстру. Цільовий outbound onboarding, де worker owner вказує лише адресу master,
є наступним етапом і ще не входить до поточного SSH MVP.

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
відповідні systemd-сервіси.

Модель навмисно не завантажується автоматично. Після інсталятора виконайте:

```bash
ollama pull MODEL_NAME
ollama list
```

Поточний SSH fallback все ще потребує, щоб master operator знав адресу worker і
додав його до реєстру. `MASTER_HOST` зберігається в конфігурації для наступного
outbound onboarding.

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

На master створити окрему пару ключів. Якщо fleet-ключ уже існує, повторно
його не створювати:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_codex_fleet \
  -C codex-fleet-master
```

Потрібен файл `~/.ssh/id_ed25519_codex_fleet.pub`. Приватний файл без `.pub`
нікому не передавати.

Якщо master уже має окремий ключ для цього fleet, можна використати його
публічну частину замість створення нової пари.

## 3. Передати ключ на worker

З master виконати:

```bash
ssh-copy-id -i ~/.ssh/id_ed25519_codex_fleet.pub WORKER_USER@WORKER_IP
```

Ввести пароль користувача `WORKER_USER` один раз. Після успішної перевірки SSH-ключа
пароль більше не потрібен.

Якщо `ssh-copy-id` відсутній:

```bash
cat ~/.ssh/id_ed25519_codex_fleet.pub | \
  ssh WORKER_USER@WORKER_IP 'umask 077; mkdir -p ~/.ssh; cat >> ~/.ssh/authorized_keys'
```

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
