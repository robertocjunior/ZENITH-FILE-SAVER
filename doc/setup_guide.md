# Guia de Configuração e Execução - Zenith File Saver

Este documento explica como configurar, compilar e executar o **Zenith File Saver** localmente ou em contêineres Docker.

---

## Requisitos Prévios

1. **Docker e Docker Compose** instalados (método recomendado).
2. **Go 1.25** instalado no host (caso queira rodar localmente sem Docker).
3. Uma **Chave de API do Gemini**. Obtenha gratuitamente em [Google AI Studio](https://aistudio.google.com/).
4. Um celular com WhatsApp ativo para escanear o QR Code de autenticação.

---

## Método Recomendado: Usando Docker Compose

Este método garante que todas as dependências do SQLite com CGO sejam resolvidas internamente na compilação do contêiner Alpine, evitando necessidade de instalar compiladores C/C++ na sua máquina host.

1. **Clonar ou baixar o repositório**.
2. **Subir os serviços**:
   ```bash
   docker compose up -d --build
   ```
3. **Acompanhar os logs de inicialização**:
   ```bash
   docker compose logs -f
   ```
4. **Acessar a interface Web**:
   Abra seu navegador em [http://localhost:8080](http://localhost:8080).

---

## Método Alternativo: Execução Local (para Desenvolvimento)

Para executar o projeto diretamente no seu computador, certifique-se de que possui o Go instalado e um compilador GCC configurado no seu PATH (necessário para compilar o SQLite via CGO).

1. **Baixar as dependências**:
   ```bash
   go mod download
   ```
2. **Compilar e rodar o servidor**:
   No Windows:
   ```powershell
   go run cmd/server/main.go
   ```
   No Linux/macOS:
   ```bash
   go run cmd/server/main.go
   ```
3. **Acessar o Painel**:
   Acesse [http://localhost:8080](http://localhost:8080).

---

## Estrutura de Diretórios Persistidos

Tanto na execução local quanto no Docker Compose, duas pastas principais serão criadas:

- **`/data`**: Armazena as sessões e configurações do aplicativo.
  - `config.json`: Chave do Gemini e ID do grupo monitorado.
  - `app.db`: SQLite com registros dos arquivos processados.
  - `whatsapp.db`: SQLite com a sessão criptografada do WhatsApp (whatsmeow).
- **`/FILES`**: Armazena fisicamente os arquivos baixados e renomeados. Mantenha essa pasta em backup.

---

## Troubleshooting / Resolução de Problemas

### 1. Erro ao compilar localmente: "CGO_ENABLED=1: cc not found"
- **Causa**: O driver SQLite nativo (`go-sqlite3`) exige um compilador C para rodar localmente.
- **Solução**: Rode o projeto utilizando o Docker Compose (que realiza a compilação de forma automática e segura dentro do contêiner Linux Alpine) ou instale o `GCC` (como MinGW no Windows ou build-essential no Ubuntu).

### 2. O QR Code não carrega ou expira na página
- **Solução**: Atualize a página do navegador. Caso o WhatsApp não se conecte após o escaneamento, use o botão **Desconectar WhatsApp** para limpar a sessão travada e gerar um novo QR Code.

### 3. Falha de processamento de arquivos: "Gemini API key is not configured"
- **Solução**: Acesse a interface Web, insira a chave do Gemini e clique em **Salvar Configurações**.
