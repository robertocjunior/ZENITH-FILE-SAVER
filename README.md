# Zenith File Saver

O **Zenith File Saver** é um aplicativo estruturado em Go que automatiza a captura, classificação, renomeação e organização de imagens e documentos enviados em um grupo específico do WhatsApp. 

O aplicativo utiliza a inteligência artificial do **Google Gemini (modelo `gemini-2.5-flash`)** para identificar os arquivos (faturas, comprovantes, notas fiscais), extrair suas datas de competência, gerar nomes amigáveis em formato slug e salvá-los de forma organizada em uma pasta física persistente. Possui um painel administrativo Web moderno em estilo glassmorphism dark mode com atualizações em tempo real (WebSockets).

---

## 🚀 Recursos Principais

- **Conexão Simples**: Autenticação fácil escaneando o QR Code na interface web (via biblioteca `whatsmeow`).
- **Monitoramento em Segundo Plano**: Ouve as mensagens do grupo selecionado e baixa imagens, PDFs, vídeos ou áudios automaticamente.
- **Suporte a Mensagens Próprias**: Processa arquivos enviados por outros membros da família e também os enviados pelo próprio celular conectado.
- **Inteligência Artificial Gemini**: Analisa o conteúdo visual/textual dos anexos para categorizá-los e extrair a data real dos documentos.
- **Organização Inteligente**: Salva os arquivos na estrutura física:
  `FILES/<ANO>/<MES>/<REMETENTE>/<DD-MM-oque_e.extensao>`
- **Painel Administrativo Web**:
  - Exibição dinâmica do QR Code de login.
  - Atualização do estado de conexão.
  - Seletor de grupo integrado (carrega os grupos da sua conta).
  - Terminal de logs em tempo real via WebSockets.
  - Tabela com histórico de arquivos processados, categoria detectada e destino.
- **Resolução de Conflitos**: Renomeia arquivos automaticamente para evitar sobreposição caso sejam enviados múltiplos comprovantes com descrições idênticas na mesma data.
- **Pronto para Docker**: Roda 100% conteinerizado usando o contêiner compilado em Go 1.25 Alpine.

---

## 📁 Estrutura do Projeto

O projeto segue as diretrizes da arquitetura estruturada de Go (Clean Code):

```text
├── .github/workflows/   # CI/CD - Build da imagem no Github
├── cmd/server/          # Ponto de entrada principal da aplicação (main.go)
├── internal/            # Lógica central da aplicação (não exportável externamente)
│   ├── config/          # Gerenciamento de configurações persistidas (JSON)
│   ├── db/              # Banco SQLite local de logs de processamento (app.db)
│   ├── gemini/          # Integração com a API do Google GenAI em Go
│   ├── storage/         # Armazenamento e organização de arquivos físicos no host
│   ├── web/             # Servidor HTTP, roteadores REST API, WebSockets e assets estáticos
│   └── whatsapp/        # Monitoramento, download e login via whatsmeow (SQLite)
├── doc/                 # Documentação detalhada do sistema
│   ├── architecture.md  # Arquitetura e diagrama de fluxo de dados
│   ├── setup_guide.md   # Guia detalhado de compilação e execução
│   └── user_guide.md    # Manual do usuário para uso diário
├── docker-compose.yml   # Definição dos volumes e containers para execução rápida
├── Dockerfile           # Imagem multi-stage Go 1.25 Alpine
├── README.md            # Este arquivo
└── go.mod / go.sum      # Dependências do projeto
```

---

## 🛠️ Início Rápido (Docker Compose)

O método mais fácil de rodar o projeto é utilizando Docker Compose. Certifique-se de ter o Docker instalado e execute:

```bash
# Iniciar o container em segundo plano
docker compose up -d --build
```

Acesse o painel em seu navegador:
👉 [**http://localhost:8080**](http://localhost:8080)

---

## 📚 Documentação Detalhada

Para mais informações sobre o funcionamento interno e instruções passo a passo, consulte os documentos na pasta `/doc`:

1. 📐 **[Arquitetura do Sistema](doc/architecture.md)**: Conheça o fluxo detalhado das mensagens e estrutura de tabelas SQLite.
2. 🔧 **[Guia de Instalação e Execução](doc/setup_guide.md)**: Instruções detalhadas para compilação local ou via Docker e resoluções de problemas (Troubleshooting).
3. 👤 **[Manual do Usuário](doc/user_guide.md)**: Saiba como configurar sua chave do Gemini, conectar seu WhatsApp e enviar os comprovantes no grupo da família.

---

## 🛡️ Aviso Legal e Licença

Este é um projeto com fins educacionais e pessoais. A biblioteca `whatsmeow` é um cliente de WhatsApp não oficial e independente. O uso desta ferramenta está sujeito aos termos de serviço do WhatsApp. Use com responsabilidade.
