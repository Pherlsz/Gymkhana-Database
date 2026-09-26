package importcatalog

import "testing"

func TestArquivoUnicoMapsEightyHeaders(t *testing.T) {
	if len(arquivoUnicoHeaders) != 80 {
		t.Fatalf("fixture must list 80 headers, got %d", len(arquivoUnicoHeaders))
	}
	targets := ApplyHeaders(ModuleProfiles, arquivoUnicoHeaders)
	if !hasTarget(targets, "full_name") {
		t.Fatalf("missing full_name: %#v", targets)
	}
	mapped := 0
	for index, target := range targets {
		if target == "" {
			t.Fatalf("unmapped arquivo-unico header %q", arquivoUnicoHeaders[index])
		}
		mapped++
	}
	if mapped != 80 {
		t.Fatalf("mapped %d/80", mapped)
	}
	if SuggestColumn(ModuleProfiles, "arquivo").TargetField != DiscardSentinel {
		t.Fatal("arquivo must discard")
	}
}

func TestMunicipalSomaColumnsDiscard(t *testing.T) {
	headers := []string{"Nome", "CPF", "SOMA_DIGITO", "SOMA_CPF", "SOMA_RG", "SOMA_NOME"}
	targets := ApplyHeaders(ModuleProfiles, headers)
	if targets[0] != "full_name" || targets[1] != "cpf" {
		t.Fatalf("identity = %#v", targets)
	}
	for _, target := range targets[2:] {
		if target != DiscardSentinel {
			t.Fatalf("SOMA_* must discard, got %#v", targets)
		}
	}
}

func TestGoogleFormsQuemIndicouDiscards(t *testing.T) {
	headers := []string{
		"Nome:", "CPF:", "Data de nascimento:", "Quem indicou?",
		"CNH (número e data da primeira emissão):",
	}
	targets := ApplyHeaders(ModuleProfiles, headers)
	if targets[0] != "full_name" || targets[1] != "cpf" || targets[2] != "birth_date" {
		t.Fatalf("forms identity = %#v", targets)
	}
	if targets[3] != DiscardSentinel {
		t.Fatalf("Quem indicou? = %q", targets[3])
	}
	if targets[4] != DocumentFieldPrefix+"cnh" {
		t.Fatalf("CNH compound = %q", targets[4])
	}
}

func TestMoldFixturesPreFillRequiredFullName(t *testing.T) {
	for _, mold := range moldFixtures {
		targets := FillUnmapped(ApplyHeaders(ModuleProfiles, mold.headers), mold.samples)
		if !hasTarget(targets, "full_name") {
			t.Fatalf("%s missing full_name: %#v", mold.name, targets)
		}
		if len(targets) != len(mold.headers) {
			t.Fatalf("%s target/header length mismatch", mold.name)
		}
	}
}

func TestFillUnmappedDiscardsExercitoIndexColumn(t *testing.T) {
	headers := []string{"Nº", "Nome", "CPF", "E-mail"}
	samples := [][]string{
		{"1", "Fixture Batch Silva", "11144477735", "a@example.test"},
		{"2", "Fixture Batch Souza", "52998224725", "b@example.test"},
		{"3", "Fixture Batch Costa", "93541134780", "c@example.test"},
	}
	targets := FillUnmapped(ApplyHeaders(ModuleProfiles, headers), samples)
	if targets[0] != DiscardSentinel {
		t.Fatalf("index col = %q want discard %#v", targets[0], targets)
	}
	if targets[1] != "full_name" || targets[2] != "cpf" || targets[3] != "email" {
		t.Fatalf("exercito map = %#v", targets)
	}
}

func TestFillUnmappedDiscardsHeaderlessPlan3Index(t *testing.T) {
	headers := []string{"Coluna 1", "Coluna 2", "Coluna 3", "Coluna 4"}
	samples := [][]string{
		{"1", "Fixture Batch Silva", "11144477735", "a@example.test"},
		{"2", "Fixture Batch Souza", "52998224725", "b@example.test"},
		{"3", "Fixture Batch Costa", "93541134780", "c@example.test"},
	}
	targets := FillUnmapped(ApplyHeaders(ModuleProfiles, headers), samples)
	if targets[0] != DiscardSentinel || targets[1] != "full_name" || targets[2] != "cpf" || targets[3] != "email" {
		t.Fatalf("plan3 = %#v", targets)
	}
}

func hasTarget(targets []string, want string) bool {
	for _, target := range targets {
		if target == want {
			return true
		}
	}
	return false
}

type moldFixture struct {
	name    string
	headers []string
	samples [][]string
}

var moldFixtures = []moldFixture{
	{
		name:    "arquivo-unico",
		headers: arquivoUnicoHeaders,
		samples: [][]string{arquivoUnicoSample},
	},
	{
		name:    "google-forms",
		headers: []string{"Nome:", "CPF:", "E-mail:", "Celular:", "Quem indicou?"},
		samples: [][]string{{"Fixture Batch Silva", "11144477735", "a@example.test", "51900000001", "indicacao"}},
	},
	{
		name:    "municipal",
		headers: []string{"Nome", "CPF", "Município", "SOMA_DIGITO", "SOMA_CPF"},
		samples: [][]string{{"Fixture Batch Silva", "11144477735", "Porto Alegre", "12", "34"}},
	},
	{
		name: "geral-i-headerless",
		headers: []string{
			"Coluna 1", "Coluna 2", "Coluna 3", "Coluna 4", "Coluna 5", "Coluna 6",
			"Coluna 7", "Coluna 8", "Coluna 9", "Coluna 10", "Coluna 11", "Coluna 12",
			"Coluna 13", "Coluna 14", "Coluna 15", "Coluna 16",
		},
		samples: [][]string{{
			"Fixture Batch Silva", "F", "39", "15/03/1985", "BRA", "1099290030",
			"11144477735", "Rua Exemplo 100", "Bairro Teste", "Cidade Fixture",
			"RS", "90000000", "batch@example.test", "51900000001", "", "5130000001",
		}},
	},
	{
		name:    "geral-iii-headed",
		headers: []string{"Nome", "Sexo", "CPF", "E-mail", "Celular", "Equipe"},
		samples: [][]string{{"Fixture Batch Silva", "F", "11144477735", "a@example.test", "51900000001", "TNC"}},
	},
	{
		name:    "tnc-title-checksum",
		headers: []string{"Nome", "CPF", "TELEFONE", "TELEFONE_2", "SOMA_CPF"},
		samples: [][]string{{"Fixture Batch Silva", "11144477735", "51900000001", "5130000001", "99"}},
	},
	{
		name:    "exercito-offset",
		headers: []string{"Nº", "Nome", "CPF", "E-mail"},
		samples: [][]string{
			{"1", "Fixture Batch Silva", "11144477735", "a@example.test"},
			{"2", "Fixture Batch Souza", "52998224725", "b@example.test"},
			{"3", "Fixture Batch Costa", "93541134780", "c@example.test"},
		},
	},
	{
		name:    "tnc-2019-ii-headerless",
		headers: []string{"Coluna 1", "Coluna 2", "Coluna 3", "Coluna 4"},
		samples: [][]string{{"Fixture Batch Silva", "11144477735", "TNC", "51900000001"}},
	},
}

var arquivoUnicoHeaders = []string{
	"arquivo", "aba", "linha_origem", "nome", "nome_social", "sexo", "tipo_sanguineo", "estado_civil",
	"data_nascimento", "cidade_nascimento", "naturalidade", "pais_nascimento", "nacionalidade",
	"endereco", "numero", "predio", "apto", "bloco", "complemento", "bairro", "cidade_reside", "uf", "cep",
	"email", "celular", "residencial", "fone_comercial", "pai", "nasc_pai", "mae", "nasc_mae",
	"casamento", "casamento_pais", "equipe", "setor", "socio_clube", "membership_type", "clube_supermercado",
	"colecao", "animal", "viagem", "plano_saude", "doador_sangue", "doador_orgaos", "cartao_bandeira", "cartao_banco",
	"veiculo_modelo", "veiculo_cor", "veiculo_placa", "veiculo_ano", "cpf", "cpf_presenca", "cpf_emissao",
	"rg", "rg_emissao", "rg_presenca", "cnh", "cnh_emissao", "cnh_presenca",
	"titulo", "titulo_zona", "titulo_secao", "titulo_presenca", "ctps", "ctps_serie", "ctps_presenca",
	"sus", "sus_presenca", "cartao_cidadao", "passaporte", "formacao", "tri", "teu", "pis",
	"observacoes", "idade", "signo", "horario_nascimento", "peculiaridade", "quem_indicou",
}

var arquivoUnicoSample = []string{
	"arquivo.xlsx", "Plan1", "4", "Fixture Batch Silva", "", "F", "O+", "solteiro",
	"15/03/1985", "Cidade Fixture", "RS", "Brasil", "BRA",
	"Rua Exemplo", "100", "", "", "", "", "Bairro Teste", "Cidade Fixture", "RS", "90000000",
	"batch@example.test", "51900000001", "5130000001", "", "Pai Fixture", "", "Mae Fixture", "",
	"", "", "TNC", "Rua", "Grêmio", "", "",
	"", "", "", "", "Sim", "Não", "", "",
	"Uno", "Prata", "HYP1983", "2014", "11144477735", "sim", "",
	"1099290030", "", "sim", "12345678900", "12/03/2010", "sim",
	"", "", "", "sim", "", "", "sim",
	"", "sim", "", "", "", "", "", "",
	"", "39", "", "", "", "",
}
