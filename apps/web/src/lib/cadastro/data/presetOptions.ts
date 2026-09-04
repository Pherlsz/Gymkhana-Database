export const BLOOD_TYPE_OPTIONS = [
  'A+',
  'A-',
  'B+',
  'B-',
  'AB+',
  'AB-',
  'O+',
  'O-',
] as const;

export const MARITAL_STATUS_OPTIONS = [
  'solteiro',
  'casado',
  'divorciado',
  'separado',
  'viuvo',
] as const;

export const GENDER_OPTIONS = ['M', 'F', 'Outro'] as const;

export const NATIONALITY_OPTIONS = [
  'Brasil',
  'Argentina',
  'Uruguai',
  'Paraguai',
  'Chile',
  'Colômbia',
  'Estados Unidos',
  'Alemanha',
  'Portugal',
  'França',
  'Japão',
  'Angola',
  'Venezuela',
] as const;

export const VEHICLE_COLOR_OPTIONS = [
  'Branco',
  'Preto',
  'Prata',
  'Cinza',
  'Vermelho',
  'Azul',
  'Verde',
  'Marrom',
  'Bege',
  'Chumbo',
  'Grafite',
] as const;

export const HEALTH_PLAN_OPTIONS = [
  'Unimed',
  'Bradesco Saúde',
  'Amil',
  'Sulmed',
  'Doctor Clin',
  'Círculo',
  'IPE Saúde',
  'Cartão de Todos',
  'Centro Clínico Gaúcho',
  'Postal Saúde',
  'Saúde Caixa',
  'AFPERGS',
  'Cabergs',
  'Odontoprev',
  'Uniodonto',
] as const;

export const TEAM_OPTIONS = [
  'Águia de Fogo',
  'Azzurra',
  'Mocó do Borogodó',
  'Revolução',
  'TNC',
  'Tatu Cascalho',
  'Tatu Pelado',
  'Templários',
] as const;

export const SECTOR_OPTIONS = [
  'Artística',
  'Charadas',
  'Construção',
  'Diversas',
  'Esportiva',
  'Fechamento',
  'Mídias',
  'Objetos',
  'Rua',
  'Secretaria',
] as const;

export const CLUB_MEMBERSHIP_OPTIONS = [
  'Grêmio',
  'Internacional',
  'Juventude',
  'Chapecoense',
] as const;

export const MEMBERSHIP_TYPE_OPTIONS = [
  'socio',
  'cadastro',
  'cartao',
  'infantil',
  'senior',
] as const;

export const CARD_BRAND_OPTIONS = [
  'Visa',
  'Mastercard',
  'Elo',
  'Hipercard',
  'Banricompras',
] as const;

export const CARD_BANK_OPTIONS = [
  'Banco do Brasil',
  'Banrisul',
  'Bradesco',
  'Caixa',
  'C6',
  'Inter',
  'Itaú',
  'Next',
  'Nubank',
  'Renner',
  'Santander',
  'Sicredi',
  'Trigg',
] as const;

export const SUPERMARKET_CLUB_OPTIONS = [
  'bonato',
  'desco',
  'eco',
  'indio',
  'mercado_muller',
  'stok_center',
  'carrefour',
] as const;

export const PET_OPTIONS = [
  'Cachorro',
  'Gato',
  'Calopsita',
  'Cavalo',
  'Coelho',
  'Pássaro',
  'Periquito',
  'Porquinho-da-índia',
  'Tartaruga',
] as const;

export function toAutoCompleteOptions(values: readonly string[]) {
  return values.map((val) => ({ value: val, label: val }));
}
