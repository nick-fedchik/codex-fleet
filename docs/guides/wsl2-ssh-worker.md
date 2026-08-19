# Windows 11 worker через WSL2

Ця інструкція перетворює Windows 11-ноутбук на Linux-вузол `codex-fleet`.
Для master він виглядає як звичайний Linux worker: окремий Linux-користувач,
SSH-ключ і Ollama.

## Що надіслати власнику хоста

Власник master передає лише:

- бажаний SSH-аліас вузла, наприклад `worker-wsl2`;
- адресу master у локальній мережі;
- публічний SSH-ключ master (`*.pub`), не приватний ключ;
- список рекомендованих Ollama-моделей;
- адресу й порт, за якими master має підключатися до WSL2.

Приватні ключі master і файли проекту власнику worker передавати не потрібно.

## Одноразове встановлення

### 1. Встановити WSL2

Відкрити PowerShell **від імені адміністратора**:

```powershell
wsl --install -d Ubuntu
```

Перезавантажити Windows. Під час першого запуску Ubuntu створити окремого
Linux-користувача `WORKER_USER` і пароль. Якщо WSL2 та Ubuntu вже встановлені,
цей крок пропустити.

### 2. Встановити SSH та Ollama в Ubuntu

У терміналі Ubuntu:

```bash
sudo apt update
sudo apt install -y openssh-server
```

Встановити Ollama за офіційною Linux-інструкцією, потім перевірити:

```bash
ollama list
ollama ps
```

Завантажити потрібні моделі, наприклад:

```bash
ollama pull MODEL_NAME
```

### 3. Додати ключ master

У Ubuntu, під користувачем `WORKER_USER`:

```bash
mkdir -p ~/.ssh
chmod 700 ~/.ssh
echo 'MASTER_PUBLIC_KEY' >> ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
```

Замість `MASTER_PUBLIC_KEY` вставити один рядок публічного ключа, отриманого від
власника master. Приватний ключ сюди не вставляти.

Запустити SSH:

```bash
sudo service ssh start
```

Якщо в цій інсталяції WSL2 увімкнено systemd, краще використовувати:

```bash
sudo systemctl enable --now ssh
```

### 4. Перевірити доступ

Власник master додає SSH-аліас, наприклад:

```sshconfig
Host worker-wsl2
    HostName WINDOWS_OR_WSL_ADDRESS
    Port SSH_PORT
    User WORKER_USER
    IdentityFile ~/.ssh/id_ed25519_fleet
    IdentitiesOnly yes
```

Після цього перевіряє з master:

```bash
ssh worker-wsl2 'hostname; id -un; ollama list; ollama ps'
```

Очікуваний результат: ім'я Windows/WSL-хоста, користувач `WORKER_USER` і список моделей.

## Що відбувається після налаштування

Власнику worker більше не потрібно запускати Codex, NATS або інші компоненти.
Він лише залишає увімкненими Windows, WSL2 та Ollama. Master періодично виконує
discovery через SSH, бачить моделі й завантаження, а потім передає prompt-задачі.

Після появи постійного fleet-agent SSH залишиться перевіреним резервним каналом,
а агент зможе передавати heartbeat без періодичного SSH-підключення.

## Важливо

- первинне встановлення WSL2 і мережевого доступу може потребувати адміністратора;
- після цього користувач працює як звичайний Linux-користувач `WORKER_USER`;
- WSL2 має бути доступний з master за стабільною адресою та портом;
- якщо вхідний SSH складно налаштувати, запасний режим — outbound-agent із WSL2,
  який сам підключається до master.
