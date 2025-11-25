#!/bin/bash

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}⚠ ATENÇÃO${NC}: Este script vai:"
echo "  - Parar todos os containers"
echo "  - Remover todos os volumes (DADOS SERÃO PERDIDOS)"
echo "  - Limpar completamente o ambiente Docker"
echo ""
read -p "Tem certeza? (digite 'sim' para confirmar): " confirm

if [ "$confirm" != "sim" ]; then
    echo "Operação cancelada."
    exit 0
fi

echo ""
echo "Limpando ambiente Docker..."

# Parar containers
echo "Parando containers..."
docker compose down

# Remover volumes
echo "Removendo volumes..."
docker compose down -v

# Remover imagens (opcional)
# docker compose down --rmi all

echo -e "\n${RED}✓${NC} Limpeza completa!"
echo ""
echo "Para recriar o ambiente, execute:"
echo "  ./scripts/docker-dev.sh setup"
