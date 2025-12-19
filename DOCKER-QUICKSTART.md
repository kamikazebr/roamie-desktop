# 🐳 Docker Quickstart - 30 Segundos para Começar

## ⚡ Setup Instantâneo

```bash
cd /home/felipenovaesrocha/Projects/roamie-desktop

# 1. Setup completo (PostgreSQL + Migrations)
./scripts/docker-dev.sh setup

# 2. Configure Resend (OBRIGATÓRIO)
nano .env
# Mude: RESEND_API_KEY=re_sua_chave_aqui

# 3. Compile
./scripts/build.sh

# 4. Rode!
./roamie-server
```

**Pronto!** Servidor rodando em http://localhost:8080 🎉

---

## 📋 Comandos Essenciais

```bash
# Iniciar PostgreSQL
./scripts/docker-dev.sh start

# Parar PostgreSQL
./scripts/docker-dev.sh stop

# Ver logs
./scripts/docker-dev.sh logs

# Acessar banco (psql)
./scripts/docker-dev.sh shell

# Reset completo
./scripts/docker-clean.sh
```

---

## 🧪 Testar

### 1. Health Check
```bash
curl http://localhost:8080/health
# Deve retornar: {"status":"healthy","service":"roamie-desktop"}
```

### 2. Solicitar código de autenticação
```bash
curl -X POST http://localhost:8080/api/auth/request-code \
  -H "Content-Type: application/json" \
  -d '{"email":"seu@email.com"}'
```

### 3. Verificar banco de dados
```bash
./scripts/docker-dev.sh shell

# No psql:
\dt                    # Listar tabelas
SELECT * FROM users;   # Ver usuários
\q                     # Sair
```

---

## 🔧 Troubleshooting Rápido

### "Porta 5432 em uso"
```bash
# Parar PostgreSQL local
sudo systemctl stop postgresql
```

### "Container já existe"
```bash
docker compose down
./scripts/docker-dev.sh start
```

### "Migrations falharam"
```bash
./scripts/migrate.sh
```

---

## 📚 Mais Informações

- `DOCKER.md` - Guia completo de Docker
- `README.md` - Documentação do projeto
- `QUICKSTART.md` - Guia de início rápido

---

**Dica**: Deixe PostgreSQL rodando no Docker e foque em desenvolver! 🚀
