package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

// ClassificationResult is the structured output from Gemini
type ClassificationResult struct {
	Category    string `json:"category"`    // "nota_fiscal", "comprovante", "fatura", "outro"
	Description string `json:"description"` // brief description slug (e.g., "energia_enel")
	Date        string `json:"date"`        // "DD-MM-YYYY"
}

// Client handles communication with Google Gemini API
type Client struct {
	modelName string
}

// NewClient creates a new Gemini client wrapper
func NewClient() *Client {
	return &Client{
		modelName: "gemini-3.5-flash", // Gemini 3.5 Flash is the stable active flash model
	}
}

// ClassifyFile sends a file to Gemini along with a classification prompt and returns the parsed result.
func (c *Client) ClassifyFile(ctx context.Context, apiKey, modelName string, fileData []byte, mimeType string) (*ClassificationResult, error) {
	if apiKey == "" {
		return nil, errors.New("Gemini API key is not configured")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	todayStr := time.Now().Format("02-01-2006")
	prompt := fmt.Sprintf(`Analise o documento ou imagem em anexo. Este arquivo representa um documento fiscal, cadastral, financeiro ou pessoal enviado no chat.
Identifique o tipo do documento e classifique-o. Exemplos de categorias comuns:
- "fatura" (contas de energia, internet, água, gás, telefone, faturas de cartão de crédito, etc.)
- "comprovante" (comprovante de transferência PIX, comprovante de pagamento de boletos, depósitos bancários, etc.)
- "nota_fiscal" (DANFE, nota fiscal eletrônica, cupom fiscal de compras)
- "documento_cadastral" (comprovante de residência, cartão CNPJ, certidões de nascimento/casamento, CNH, RG, CPF, etc.)
- "documento_fiscal" (declarações de impostos de renda, guias de arrecadação tributária, informes de rendimentos, etc.)
- "contrato" (contratos de aluguel, contratos de trabalho, contratos sociais, acordos de serviço, etc.)
- "extrato" (extratos bancários consolidados, faturas de corretoras, relatórios financeiros, etc.)
- "irrelevante" (fotos pessoais, selfies, animais, memes, fotos de objetos aleatórios ou qualquer imagem/documento sem relevância cadastral ou fiscal)
- "outro" (qualquer outro tipo de documento útil)

Caso o documento se encaixe melhor em uma outra classificação específica que não está na lista acima, sinta-se livre para gerar uma nova categoria curta e representativa (utilize sempre letras minúsculas em formato snake_case, ex: "comprovante_vacina", "documento_veiculo").

Extraia:
1. "category": A categoria identificada (em letras minúsculas, snake_case).
2. "description": Uma descrição curta (slug, em português, tudo minúsculo, substituindo espaços por hífens, sem acentos ou caracteres especiais, máximo 3 palavras) identificando o assunto principal do documento (ex: "energia-enel", "contrato-aluguel", "declaracao-irpf", "residencia-sabesp", "cnh-roberto").
3. "date": A data correspondente ao documento ou transação no formato "DD-MM-AAAA". Se NÃO houver nenhuma data clara no documento, utilize a data de hoje: "%s".

Você DEVE responder APENAS com um objeto JSON válido, sem formatação markdown (como blocos de código JSON), sem textos explicativos adicionais.
O formato do JSON de resposta deve ser exatamente:
{
  "category": "categoria-detectada",
  "description": "slug-da-descricao",
  "date": "DD-MM-AAAA"
}
`, todayStr)

	parts := []*genai.Part{
		{Text: prompt},
		{InlineData: &genai.Blob{
			Data:     fileData,
			MIMEType: mimeType,
		}},
	}

	config := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
	}

	if modelName == "" {
		modelName = c.modelName
	}

	resp, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{{Parts: parts}}, config)
	if err != nil {
		return nil, fmt.Errorf("Gemini API request failed: %w", err)
	}

	respText := resp.Text()
	if respText == "" {
		return nil, errors.New("received empty response from Gemini")
	}

	var result ClassificationResult
	if err := json.Unmarshal([]byte(cleanJSON(respText)), &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini JSON response: %w. Raw response: %s", err, respText)
	}

	// Basic validation / sanitization of result
	if result.Category == "" {
		result.Category = "outro"
	}
	if result.Description == "" {
		result.Description = "documento-desconhecido"
	}
	if result.Date == "" {
		result.Date = todayStr
	}

	return &result, nil
}

// cleanJSON finds the JSON block between the first '{' and the last '}'
func cleanJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || start >= end {
		return s
	}
	return s[start : end+1]
}
