# Remote Upgrade System

Sistema de upgrade remoto que permite acionar atualizações de clientes via API, com polling automático pelo daemon.

## Arquitetura

Similar ao sistema de diagnósticos remotos, usa padrão server-as-proxy:

```
┌──────────────┐     POST trigger-upgrade      ┌──────────────┐
│ Admin/Mobile │ ────────────────────────────> │    Server    │
└──────────────┘                               └──────────────┘
                                                       │
                                                       ▼
                                              ┌─────────────────┐
                                              │    Firestore    │
                                              │  upgrade_       │
                                              │  requests/      │
                                              │  {device}/      │
                                              │  pending/       │
                                              │  {request_id}   │
                                              └─────────────────┘
                                                       │
                                                       │ Poll (30s)
┌──────────────┐                                      │
│   Daemon     │ <────────────────────────────────────┘
│   (Client)   │
└──────────────┘
       │
       │ Run upgrade
       ▼
┌──────────────┐     POST result                ┌──────────────┐
│   Upgrade    │ ─────────────────────────────> │    Server    │
│   Module     │                                 └──────────────┘
└──────────────┘                                        │
                                                        ▼
                                               ┌─────────────────┐
                                               │    Firestore    │
                                               │  upgrade_       │
                                               │  results/       │
                                               │  {device}/      │
                                               │  {request_id}   │
                                               └─────────────────┘
```

## Firestore Collections

### `upgrade_requests/{device_name}/pending/{request_id}`
```json
{
  "request_id": "uuid",
  "device_id": "device_name",
  "user_id": "uuid",
  "requested_at": "timestamp",
  "requested_by": "api|mobile_app|dashboard",
  "status": "pending",
  "target_version": "optional specific version"
}
```

### `upgrade_results/{device_name}/results/{request_id}`
```json
{
  "request_id": "uuid",
  "device_id": "device_name",
  "ran_at": "timestamp",
  "success": true|false,
  "previous_version": "v0.0.10",
  "new_version": "v0.0.11",
  "error_message": "optional error details"
}
```

## API Endpoints

### Server Endpoints

#### Trigger Upgrade
```bash
POST /api/devices/{device_id}/trigger-upgrade
Authorization: Bearer {jwt}
Content-Type: application/json

{
  "target_version": "v0.0.11"  # Optional
}

Response:
{
  "request_id": "uuid",
  "device_id": "uuid",
  "device_name": "pop-os",
  "status": "pending",
  "target_version": "v0.0.11",
  "message": "Upgrade request created. Device daemon will process it within 30 seconds."
}
```

#### Get Upgrade Result
```bash
GET /api/devices/{device_id}/upgrade/{request_id}
Authorization: Bearer {jwt}

Response:
{
  "request_id": "uuid",
  "device_id": "pop-os",
  "ran_at": "2026-01-17T12:00:00Z",
  "success": true,
  "previous_version": "v0.0.10",
  "new_version": "v0.0.11",
  "error_message": ""
}
```

### Daemon Endpoints (Server-as-Proxy)

#### Get Pending Upgrades
```bash
GET /api/devices/upgrades/pending
Authorization: Bearer {jwt}

Response:
{
  "pending_upgrades": [
    {
      "request_id": "uuid",
      "device_id": "uuid",
      "device_name": "pop-os",
      "target_version": "v0.0.11"
    }
  ],
  "count": 1
}
```

#### Upload Upgrade Result
```bash
POST /api/devices/upgrades/result
Authorization: Bearer {jwt}
Content-Type: application/json

{
  "request_id": "uuid",
  "device_id": "pop-os",
  "success": true,
  "previous_version": "v0.0.10",
  "new_version": "v0.0.11",
  "error_message": "",
  "ran_at": "2026-01-17T12:00:00Z"
}
```

## Client Implementation

### Daemon Polling

O daemon verifica upgrades pendentes:
- **No startup**: Verifica imediatamente ao iniciar
- **A cada 30 segundos**: Polling contínuo via ticker

Fluxo de execução:
1. Busca upgrades pendentes via `GET /api/devices/upgrades/pending`
2. Para cada upgrade pendente:
   - Executa `upgrade.CheckForUpdates()`
   - Se atualização disponível, executa `upgrade.Upgrade()`
   - Faz upload do resultado (sucesso/falha) via `POST /api/devices/upgrades/result`
   - Se sucesso, reinicia daemon com novo binário

### Código Principal

**`internal/client/daemon/refresh.go`**:
```go
// Ticker para upgrades remotos (30 segundos)
remoteUpgradeTicker := time.NewTicker(30 * time.Second)

// Verifica imediatamente no startup
if err := checkAndRunUpgrade(); err != nil {
    log.Printf("Initial upgrade check failed: %v", err)
}

// No loop principal
case <-remoteUpgradeTicker.C:
    if err := checkAndRunUpgrade(); err != nil {
        log.Printf("Remote upgrade check failed: %v", err)
    }
```

**`checkAndRunUpgrade()`**:
```go
func checkAndRunUpgrade() error {
    // 1. Carrega config e JWT
    // 2. Busca upgrades pendentes
    // 3. Para cada upgrade:
    //    - Verifica se há atualização disponível
    //    - Executa upgrade
    //    - Faz upload do resultado
    //    - Reinicia daemon (se sucesso)
    return nil
}
```

## Como Usar

### 1. Acionar upgrade remoto via API

```bash
# Obter device_id
DEVICE_ID="18b01818-ce05-442c-90f4-a4c072e7d517"

# Obter JWT
JWT=$(jq -r '.jwt' ~/.config/roamie/config.json)

# Acionar upgrade
curl -X POST "http://178.156.133.88:8081/api/devices/$DEVICE_ID/trigger-upgrade" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json"
```

### 2. Aguardar daemon processar

O daemon processa em até 30 segundos:
- Verifica atualização disponível
- Faz download do novo binário
- Verifica checksum
- Substitui binário antigo
- Reinicia daemon

### 3. Verificar resultado

```bash
# Obter request_id do passo 1
REQUEST_ID="uuid-from-trigger-response"

# Buscar resultado
curl "http://178.156.133.88:8081/api/devices/$DEVICE_ID/upgrade/$REQUEST_ID" \
  -H "Authorization: Bearer $JWT"
```

## Script de Teste

```bash
./scripts/test-remote-upgrade.sh [device-id]
```

O script:
1. Aciona upgrade remoto
2. Aguarda até 2 minutos pelo resultado
3. Exibe resultado (sucesso/falha)

## Logs do Daemon

Ver logs de upgrade no daemon:
```bash
# Daemon rodando em systemd
journalctl --user -f | grep -i upgrade

# Daemon rodando manualmente
# Saída no terminal
```

## Casos de Uso

### 1. Upgrade Remoto de Dispositivo Específico
Administrador pode acionar upgrade de um dispositivo específico:
```bash
curl -X POST "$SERVER/api/devices/$DEVICE_ID/trigger-upgrade" \
  -H "Authorization: Bearer $JWT"
```

### 2. Upgrade em Massa (futuro)
Mobile app pode acionar upgrade de todos os dispositivos do usuário:
```bash
# Para cada device do usuário:
for device in $(jq -r '.devices[].id' devices.json); do
  curl -X POST "$SERVER/api/devices/$device/trigger-upgrade" \
    -H "Authorization: Bearer $JWT"
done
```

### 3. Rollback de Versão Específica (futuro)
Especificar versão alvo:
```bash
curl -X POST "$SERVER/api/devices/$DEVICE_ID/trigger-upgrade" \
  -H "Authorization: Bearer $JWT" \
  -H "Content-Type: application/json" \
  -d '{"target_version": "v0.0.9"}'
```

## Diferenças do Auto-Upgrade

| Característica | Auto-Upgrade (24h) | Remote Upgrade (30s) |
|----------------|-------------------|---------------------|
| **Polling** | 24 horas | 30 segundos |
| **Trigger** | Automático | Manual via API |
| **Controle** | Config local | Remoto (admin/mobile) |
| **Uso** | Manutenção rotineira | Emergências, patches críticos |
| **Versão alvo** | Sempre latest | Pode especificar versão |

## Próximos Passos

1. **Testar com dispositivo real**:
   - Build + deploy para produção
   - Acionar upgrade remoto
   - Verificar logs do daemon
   - Confirmar resultado no Firestore

2. **Integrar no mobile app**:
   - Adicionar botão "Force Upgrade" por device
   - Exibir status de upgrade pendente
   - Mostrar histórico de upgrades

3. **Monitoramento**:
   - Dashboard mostrando upgrades em andamento
   - Alertas para upgrades que falharam
   - Estatísticas de versões por device

## Troubleshooting

### Upgrade não processa
1. Verificar se daemon está rodando: `ps aux | grep roamie`
2. Verificar logs: `journalctl --user -f | grep -i upgrade`
3. Verificar Firestore: Existe request em `upgrade_requests/{device}/pending/`?

### Upgrade falha
1. Ver error_message no resultado
2. Verificar permissões do binário
3. Verificar espaço em disco
4. Verificar conexão com GitHub (download do binário)

### Resultado não aparece no Firestore
1. Verificar JWT válido
2. Verificar device_id (usar DeviceName, não UUID)
3. Ver logs do daemon para erros de upload
