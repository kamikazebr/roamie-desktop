# Guia de Início Rápido - Roamie VPN

## 🚀 Setup Rápido com Docker (RECOMENDADO)

**Forma mais rápida de começar!** Não precisa instalar PostgreSQL.

```bash
cd /home/felipenovaesrocha/Projects/roamie-vpn

# Setup completo em 1 comando
./scripts/docker-dev.sh setup

# Configure sua chave Resend
nano .env  # Edite RESEND_API_KEY

# Compile e rode
./scripts/build.sh
./roamie-server
```

✅ **Pronto!** PostgreSQL está rodando no Docker.

📖 **Documentação completa**: Veja `DOCKER.md` para mais detalhes.

---

## Setup Alternativo (PostgreSQL Local)

### 1. Preparar Banco de Dados

```bash
# Instalar PostgreSQL (se necessário)
sudo apt-get install postgresql postgresql-contrib

# Criar banco de dados
sudo -u postgres psql
CREATE DATABASE wireguard_vpn;
CREATE USER wireguard WITH PASSWORD 'wireguard';
GRANT ALL PRIVILEGES ON DATABASE wireguard_vpn TO wireguard;
\q
```

### 2. Configurar Ambiente

```bash
cd /home/felipenovaesrocha/Projects/roamie-vpn

# Configurar variáveis de ambiente
cp .env.example .env
nano .env  # Edite com suas configurações
```

**Configurações mínimas necessárias:**
```
DATABASE_URL=postgresql://wireguard:wireguard@localhost:5432/wireguard_vpn?sslmode=disable
JWT_SECRET=mude-isso-para-um-secret-forte
RESEND_API_KEY=re_sua_chave_aqui
FROM_EMAIL=noreply@seudominio.com
WG_SERVER_PUBLIC_ENDPOINT=seu-ip-publico:51820
```

### 3. Rodar Migrations

```bash
./scripts/migrate.sh
```

### 4. Compilar

```bash
./scripts/build.sh
```

### 5. Iniciar Servidor (Desenvolvimento)

```bash
# Sem WireGuard (apenas testar API)
./roamie-server

# Com WireGuard (requer root e setup prévio)
sudo ./scripts/setup-server.sh
sudo ./roamie-server
```

### 6. Testar Cliente

```bash
# Login
export API_URL=http://localhost:8080
./roamie login

# Adicionar dispositivo
./roamie device add

# Listar dispositivos
./roamie device list
```

## Setup para Produção (VPS)

### 1. Servidor VPS

```bash
# SSH no servidor
ssh root@seu-vps

# Instalar dependências
apt-get update
apt-get install -y postgresql wireguard git golang-go

# Clone o projeto
git clone https://github.com/felipenovaesrocha/roamie-vpn
cd roamie-vpn

# Configure .env
cp .env.example .env
nano .env  # Configure com suas credenciais

# Setup completo
./scripts/setup-server.sh
./scripts/migrate.sh
./scripts/build.sh

# Inicie o servidor
sudo ./roamie-server
```

### 2. Cliente (Seu Computador)

```bash
# Compile o cliente
go build -o roamie ./cmd/client

# Configure API URL
export API_URL=https://seu-vps-ip:8080

# Login e adicione device
./roamie login
./roamie device add
sudo ./roamie connect
```

## Testando o Sistema

### Teste 1: Autenticação

```bash
curl -X POST http://localhost:8080/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"seu@email.com"}'

# Verifique seu email e use o código
curl -X POST http://localhost:8080/api/auth/verify-code \
  -H "Content-Type: application/json" \
  -d '{"email":"seu@email.com","code":"123456"}'
```

### Teste 2: Health Check

```bash
curl http://localhost:8080/health
```

### Teste 3: Network Scan

```bash
curl http://localhost:8080/api/admin/network/scan
```

## Estrutura de Arquivos Importantes

```
roamie-vpn/
├── .env                    # Configurações (criar a partir de .env.example)
├── roamie-server              # Binário do servidor (após build)
├── roamie              # Binário do cliente (após build)
├── scripts/
│   ├── setup-server.sh    # Setup WireGuard (root)
│   ├── migrate.sh         # Rodar migrations
│   └── build.sh           # Compilar projeto
└── migrations/            # SQL migrations
```

## Troubleshooting Comum

### "DATABASE_URL not set"
```bash
# Certifique-se de que .env existe
cp .env.example .env
nano .env
```

### "Failed to connect to database"
```bash
# Verifique PostgreSQL
sudo systemctl status postgresql
sudo systemctl start postgresql

# Teste conexão
psql postgresql://wireguard:wireguard@localhost:5432/wireguard_vpn
```

### "WireGuard interface does not exist"
```bash
# Execute setup
sudo ./scripts/setup-server.sh

# Ou crie manualmente
sudo ip link add dev wg0 type wireguard
```

### "Permission denied" ao conectar
```bash
# Use sudo
sudo ./roamie connect
```

## Próximos Passos

1. Configure um domínio e HTTPS (recomendado para produção)
2. Configure firewall (UFW) adequadamente
3. Configure systemd service para auto-start
4. Implemente backup do banco de dados
5. Configure monitoramento (logs, metrics)

## Comandos Úteis

```bash
# Ver peers conectados
sudo wg show

# Ver logs do servidor
journalctl -u wg-quick@wg0 -f

# Verificar rotas
ip route

# Testar conectividade VPN
ping 10.100.0.1
```

## Obtendo Ajuda

- **README**: Documentação completa
- **GitHub Issues**: https://github.com/felipenovaesrocha/roamie-vpn/issues
- **Logs do servidor**: Verifique output do `./roamie-server`

## Exemplo de Fluxo Completo

```bash
# 1. Login
./roamie login
# Email: user@example.com
# Code: 123456

# 2. Adicionar dispositivo
./roamie device add
# Device name: Laptop

# 3. Conectar
sudo ./roamie connect

# 4. Verificar status
./roamie status

# 5. Testar conectividade
ping 10.100.0.1

# 6. Desconectar
sudo ./roamie disconnect
```
