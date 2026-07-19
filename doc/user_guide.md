# Manual do Usuário - Zenith File Saver

Este guia explica como usar o aplicativo **Zenith File Saver** no dia a dia para monitorar o grupo do WhatsApp e organizar os arquivos da família.

---

## Fluxo de Primeiro Acesso

1. Certifique-se de que o aplicativo esteja rodando e acesse [http://localhost:8080](http://localhost:8080) em seu navegador.
2. Na seção **Conexão WhatsApp**, você verá um spinner carregando e, em seguida, um **QR Code**.
3. Abra o WhatsApp no seu celular:
   - Vá em **Configurações** / **Aparelhos Conectados** / **Conectar um Aparelho**.
   - Aponte a câmera para a tela do computador e escaneie o QR Code.
4. Após o escaneamento:
   - O painel web se atualizará automaticamente para o estado **Conectado** (cor verde).
   - O QR Code desaparecerá e as opções de configurações à direita serão habilitadas.

---

## Configuração do Monitoramento

Para que o bot comece a processar os arquivos, você precisa preencher as duas configurações obrigatórias no painel:

1. **Chave de API do Gemini**:
   - Insira sua chave `AIzaSy...` no campo correspondente.
2. **Grupo para Monitorar**:
   - O sistema irá carregar todos os seus grupos do WhatsApp automaticamente no dropdown.
   - Escolha o grupo da família onde você e seus pais irão mandar os arquivos.
   - Se o grupo acabou de ser criado e não aparece, clique no botão **Recarregar Grupos**.
3. **Salvar**:
   - Clique em **Salvar Configurações** para ativar o monitoramento.

---

## Como Usar no WhatsApp

Com o monitor configurado, o bot funcionará de forma 100% autônoma e em segundo plano.

### 1. Enviando Arquivos
Você, seu pai ou sua mãe podem enviar arquivos no grupo selecionado:
- **Fotos / Imagens**: Fotos de comprovantes fiscais, recibos de supermercado ou comprovantes de PIX tiradas com a câmera do celular ou prints de tela.
- **Documentos**: PDFs de faturas de luz (Enel, Light, etc.), contas de internet, taxas de condomínio ou faturas de cartão.
- **Outros formatos**: Fotos, comprovantes em PDF, etc.

> [!NOTE]
> O bot monitora mensagens de mídia enviadas por qualquer participante do grupo, **incluindo você mesmo** (mensagens enviadas do seu próprio número de WhatsApp).

### 2. Acompanhando o Processamento
Na tela do painel web, você pode acompanhar todo o progresso em tempo real:
- O **Console de Eventos** mostrará linhas indicando que o arquivo foi detectado, baixado e classificado pelo Gemini.
- A tabela **Arquivos Processados** atualizará na mesma hora, exibindo os detalhes do remetente, a categoria inferida, o novo nome do arquivo e o caminho onde foi gravado.

---

## Estrutura Física dos Arquivos Salvos

Os arquivos salvos serão estruturados na pasta `/FILES` no host seguindo a seguinte lógica:

`FILES/ <ANO> / <MES> / <NOME_DO_SENDER> / <DD-MM-oque_e.extensao>`

### Exemplos de organização:

1. Se o usuário **Robert** enviou no grupo uma fatura de luz da Enel datada de 12 de Julho de 2026:
   - **Destino**: `FILES/2026/07/Robert/12-07-energia-enel.pdf`

2. Se a **Mãe** enviou um comprovante de transferência PIX datado de 15 de Julho de 2026:
   - **Destino**: `FILES/2026/07/Mae/15-07-transferencia-pix.jpg`

3. Se o **Pai** enviou uma nota fiscal de supermercado datada de 20 de Julho de 2026:
   - **Destino**: `FILES/2026/07/Pai/20-07-supermercado-carrefour.png`

> [!TIP]
> Caso dois comprovantes idênticos ou parecidos com o mesmo nome e mesma data sejam enviados (por exemplo, dois PIXs no mesmo dia), o sistema salvará o segundo arquivo como `15-07-transferencia-pix-1.jpg`, resolvendo colisões de nome automaticamente sem perder nenhum arquivo.
