#!/bin/bash

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

function log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

function log_error() {
    echo -e "${RED}✗${NC} $1"
}

function log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

function check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker não está instalado"
        echo "Instale Docker: https://docs.docker.com/engine/install/"
        exit 1
    fi

    if ! docker compose version &> /dev/null; then
        log_error "Docker Compose não está disponível"
        exit 1
    fi
}

function wait_for_postgres() {
    log_info "Aguardando PostgreSQL ficar pronto..."

    max_attempts=30
    attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if docker compose exec -T postgres pg_isready -U wireguard -d wireguard_vpn &> /dev/null; then
            log_success "PostgreSQL está pronto!"
            return 0
        fi

        attempt=$((attempt + 1))
        echo -n "."
        sleep 1
    done

    log_error "PostgreSQL não ficou pronto após ${max_attempts}s"
    return 1
}

function setup() {
    log_info "=== Setup Ambiente Docker ==="

    check_docker

    # Criar .env se não existir
    if [ ! -f .env ]; then
        log_info "Criando .env a partir de .env.docker..."
        cp .env.docker .env
        log_success ".env criado"
    else
        log_warning ".env já existe, não foi sobrescrito"
    fi

    # Subir containers
    log_info "Iniciando containers Docker..."
    docker compose up -d

    # Aguardar PostgreSQL
    wait_for_postgres || exit 1

    # Executar migrations
    log_info "Executando migrations..."
    if ./scripts/migrate.sh; then
        log_success "Migrations executadas com sucesso!"
    else
        log_error "Erro ao executar migrations"
        exit 1
    fi

    echo ""
    log_success "=== Setup Completo! ==="
    echo ""
    echo "Próximos passos:"
    echo "  1. Configure sua RESEND_API_KEY no arquivo .env"
    echo "  2. Compile o projeto: ./scripts/build.sh"
    echo "  3. Inicie o servidor: ./roamie-server"
    echo ""
    echo "Comandos úteis:"
    echo "  ./scripts/docker-dev.sh logs    - Ver logs do PostgreSQL"
    echo "  ./scripts/docker-dev.sh stop    - Parar containers"
    echo "  ./scripts/docker-dev.sh start   - Iniciar containers"
    echo "  ./scripts/docker-clean.sh       - Reset completo"
}

function start() {
    check_docker
    log_info "Iniciando containers..."
    docker compose up -d
    wait_for_postgres
    log_success "Containers iniciados!"
}

function stop() {
    check_docker
    log_info "Parando containers..."
    docker compose stop
    log_success "Containers parados!"
}

function restart() {
    stop
    start
}

function logs() {
    check_docker
    docker compose logs -f postgres
}

function status() {
    check_docker
    docker compose ps
}

function shell() {
    check_docker
    log_info "Abrindo shell no PostgreSQL..."
    docker compose exec postgres psql -U wireguard -d wireguard_vpn
}

function help() {
    echo "Uso: $0 [comando]"
    echo ""
    echo "Comandos disponíveis:"
    echo "  setup      - Setup inicial completo (containers + migrations)"
    echo "  start      - Iniciar containers"
    echo "  stop       - Parar containers"
    echo "  restart    - Reiniciar containers"
    echo "  logs       - Ver logs do PostgreSQL"
    echo "  status     - Ver status dos containers"
    echo "  shell      - Abrir psql no banco"
    echo "  help       - Mostrar esta ajuda"
    echo ""
}

# Main
case "${1:-help}" in
    setup)
        setup
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    logs)
        logs
        ;;
    status)
        status
        ;;
    shell)
        shell
        ;;
    help|--help|-h)
        help
        ;;
    *)
        log_error "Comando desconhecido: $1"
        help
        exit 1
        ;;
esac
