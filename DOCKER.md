# Guia Docker - Roamie VPN

## 🐳 Desenvolvimento com Docker

Este guia mostra como usar Docker para desenvolvimento local, sem precisar instalar PostgreSQL manualmente.

---

## 📋 Pré-requisitos

- Docker instalado ([Guia de instalação](https://docs.docker.com/engine/install/))
- Docker Compose (geralmente vem com Docker Desktop)

### Verificar instalação:
```bash
docker --version
docker compose version
```

---

## 🚀 Setup Inicial (Primeira vez)

### Passo 1: Setup automático
```bash
cd /home/felipenovaesrocha/Projects/roamie-desktop

# Setup completo: sobe PostgreSQL + executa migrations
./scripts/docker-dev.sh setup
```

Isso vai:
1. ✅ Criar arquivo `.env` a partir de `.env.docker`
2. ✅ Subir container PostgreSQL
3. ✅ Aguardar PostgreSQL ficar pronto
4. ✅ Executar todas as migrations

### Passo 2: Configure Resend API Key
```bash
nano .env
# Edite a linha:
# RESEND_API_KEY=re_sua_chave_real_aqui
```

### Passo 3: Compile e rode
```bash
./scripts/build.sh
./roamie-server
```

**Pronto!** O servidor está rodando com PostgreSQL no Docker.

---

## 📦 O que está rodando?

```yaml
Serviços Docker:
├── postgres (PostgreSQL 15)
│   ├── Usuário: wireguard
│   ├── Senha: wireguard
│   ├── Database: wireguard_vpn
│   ├── Porta: 5432 (exposta no host)
│   └── Volume: persistente
```

---

## 🎮 Comandos Disponíveis

### Gerenciamento básico
```bash
# Iniciar containers
./scripts/docker-dev.sh start

# Parar containers
./scripts/docker-dev.sh stop

# Reiniciar containers
./scripts/docker-dev.sh restart

# Ver status
./scripts/docker-dev.sh status
```

### Logs e debugging
```bash
# Ver logs do PostgreSQL (tempo real)
./scripts/docker-dev.sh logs

# Abrir shell SQL no banco
./scripts/docker-dev.sh shell
```

### Reset completo
```bash
# Limpar TUDO (dados serão perdidos!)
./scripts/docker-clean.sh

# Recriar do zero
./scripts/docker-dev.sh setup
```

---

## 🔍 Troubleshooting

### Problema: "Container já existe"
```bash
# Parar e remover containers antigos
docker compose down
./scripts/docker-dev.sh start
```

### Problema: "Porta 5432 já em uso"
```bash
# Verificar se PostgreSQL local está rodando
sudo systemctl stop postgresql

# Ou mude a porta no docker-compose.yml:
# ports:
#   - "5433:5432"
# E atualize DATABASE_URL no .env para usar porta 5433
```

### Problema: "Migrations falharam"
```bash
# Rodar migrations manualmente
./scripts/migrate.sh

# Ou acessar o banco e verificar
./scripts/docker-dev.sh shell
\dt  -- Listar tabelas
```

### Problema: "Não consigo conectar ao banco"
```bash
# Verificar se container está rodando
docker compose ps

# Ver logs
./scripts/docker-dev.sh logs

# Testar conexão
docker compose exec postgres pg_isready -U wireguard
```

---

## 💾 Dados Persistentes

Os dados do PostgreSQL são armazenados em um **volume Docker** chamado `postgres_data`.

### Ver volumes
```bash
docker volume ls | grep culodi
```

### Backup do banco
```bash
docker compose exec postgres pg_dump -U wireguard wireguard_vpn > backup.sql
```

### Restaurar backup
```bash
cat backup.sql | docker compose exec -T postgres psql -U wireguard -d wireguard_vpn
```

---

## 🧪 Testando o Sistema

### 1. Verificar PostgreSQL
```bash
./scripts/docker-dev.sh shell

# No psql:
\dt              -- Listar tabelas
SELECT * FROM users;
\q               -- Sair
```

### 2. Testar API
```bash
# Health check
curl http://localhost:8080/health

# Request auth code
curl -X POST http://localhost:8080/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"seu@email.com"}'
```

### 3. Verificar logs
```bash
# PostgreSQL
./scripts/docker-dev.sh logs

# Servidor (em outro terminal)
./roamie-server
```

---

## 🎯 Fluxo de Trabalho Diário

```bash
# 1. Iniciar ambiente
./scripts/docker-dev.sh start

# 2. Rodar servidor
./roamie-server

# 3. Desenvolver...

# 4. Parar ao final do dia
./scripts/docker-dev.sh stop
```

---

## 📊 Comparação: Docker vs PostgreSQL Local

| Aspecto | Docker | PostgreSQL Local |
|---------|--------|------------------|
| Instalação | ✅ Rápida (1 comando) | ⚠️ Manual |
| Isolamento | ✅ Completo | ❌ Compartilha sistema |
| Reset | ✅ Fácil (1 comando) | ⚠️ Manual |
| Múltiplas versões | ✅ Simples | ⚠️ Complexo |
| Performance | ⚠️ Ligeiramente mais lenta | ✅ Nativa |
| Produção | ⚠️ Não recomendado* | ✅ Recomendado |

*Para produção, use PostgreSQL gerenciado (AWS RDS, etc) ou instalação nativa no servidor.

---

## 🔧 Configurações Avançadas

### Habilitar pgAdmin (Interface Gráfica)

Edite `docker-compose.yml` e descomente a seção `pgadmin`:

```yaml
pgadmin:
  image: dpage/pgadmin4:latest
  # ... (já está no arquivo)
```

Depois:
```bash
docker compose up -d
# Acesse: http://localhost:5050
# Login: admin@roamie.com / admin
```

### Mudar configurações do PostgreSQL

Edite `docker-compose.yml` e adicione em `environment`:
```yaml
POSTGRES_MAX_CONNECTIONS: 200
POSTGRES_SHARED_BUFFERS: 256MB
```

### Usar rede customizada

Útil se você tem outros serviços Docker:
```yaml
networks:
  roamie-desktop-network:
    external: true
```

---

## 📝 Variáveis de Ambiente (.env.docker)

Todas as configurações estão em `.env.docker` (copiado para `.env` no setup):

```bash
# Database (aponta para Docker)
DATABASE_URL=postgresql://wireguard:wireguard@localhost:5432/wireguard_vpn

# Resend (VOCÊ PRECISA CONFIGURAR)
RESEND_API_KEY=re_sua_chave

# WireGuard (ajuste para seu IP público)
WG_SERVER_PUBLIC_ENDPOINT=localhost:51820
```

---

## ✅ Checklist de Verificação

Antes de começar a desenvolver:

- [ ] Docker instalado e rodando
- [ ] `./scripts/docker-dev.sh setup` executado com sucesso
- [ ] RESEND_API_KEY configurada no `.env`
- [ ] PostgreSQL respondendo (`./scripts/docker-dev.sh shell`)
- [ ] Migrations aplicadas (4 tabelas criadas)
- [ ] Projeto compilado (`./scripts/build.sh`)
- [ ] Health check respondendo (`curl http://localhost:8080/health`)

---

## 🆘 Problemas Comuns

### "Cannot connect to Docker daemon"
```bash
# Inicie o Docker
sudo systemctl start docker

# Ou Docker Desktop no macOS/Windows
```

### "Permission denied" ao executar scripts
```bash
chmod +x scripts/*.sh
```

### "Database does not exist"
```bash
# Recriar do zero
./scripts/docker-clean.sh
./scripts/docker-dev.sh setup
```

---

## 📚 Recursos

- [Documentação Docker](https://docs.docker.com/)
- [PostgreSQL no Docker](https://hub.docker.com/_/postgres)
- [Docker Compose](https://docs.docker.com/compose/)

---

**Pronto para desenvolver!** 🎉

Para mais informações, veja:
- `README.md` - Documentação completa do projeto
- `QUICKSTART.md` - Guia de início rápido
