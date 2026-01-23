-- 99_stable_keys.sql
-- Claves estables para continentes, categorías y países + índices

-- ===== Continentes: code estable =====
ALTER TABLE continentes
  ADD COLUMN IF NOT EXISTS code TEXT UNIQUE;

UPDATE continentes SET code = CASE nombre
  WHEN 'África'  THEN 'AF'
  WHEN 'América' THEN 'AM'
  WHEN 'Asia'    THEN 'AS'
  WHEN 'Europa'  THEN 'EU'
  WHEN 'Oceanía' THEN 'OC'
  WHEN 'Antártida' THEN 'AN'
END
WHERE code IS NULL;

-- ===== Categorías: slug estable =====
ALTER TABLE categorias
  ADD COLUMN IF NOT EXISTS slug TEXT UNIQUE;

UPDATE categorias
SET slug = lower(regexp_replace(nombre, '\s+', '-', 'g'))
WHERE slug IS NULL;

-- ===== Países: ISO2 / ISO3 =====
ALTER TABLE paises
  ADD COLUMN IF NOT EXISTS iso2 CHAR(2) UNIQUE,
  ADD COLUMN IF NOT EXISTS iso3 CHAR(3) UNIQUE;

-- Mapeo ISO-3166 (alpha-2 / alpha-3) para TODOS los países de 04-paises.sql
-- Europa/Asia/África/Américas/Oceanía (nombres en español tal como en tu seed)

UPDATE paises SET iso2='AF', iso3='AFG' WHERE nombre='Afganistán' AND iso2 IS NULL;
UPDATE paises SET iso2='AL', iso3='ALB' WHERE nombre='Albania' AND iso2 IS NULL;
UPDATE paises SET iso2='DE', iso3='DEU' WHERE nombre='Alemania' AND iso2 IS NULL;
UPDATE paises SET iso2='AD', iso3='AND' WHERE nombre='Andorra' AND iso2 IS NULL;
UPDATE paises SET iso2='AO', iso3='AGO' WHERE nombre='Angola' AND iso2 IS NULL;
UPDATE paises SET iso2='AG', iso3='ATG' WHERE nombre='Antigua y Barbuda' AND iso2 IS NULL;
UPDATE paises SET iso2='SA', iso3='SAU' WHERE nombre='Arabia Saudita' AND iso2 IS NULL;
UPDATE paises SET iso2='DZ', iso3='DZA' WHERE nombre='Argelia' AND iso2 IS NULL;
UPDATE paises SET iso2='AR', iso3='ARG' WHERE nombre='Argentina' AND iso2 IS NULL;
UPDATE paises SET iso2='AM', iso3='ARM' WHERE nombre='Armenia' AND iso2 IS NULL;
UPDATE paises SET iso2='AU', iso3='AUS' WHERE nombre='Australia' AND iso2 IS NULL;
UPDATE paises SET iso2='AT', iso3='AUT' WHERE nombre='Austria' AND iso2 IS NULL;
UPDATE paises SET iso2='AZ', iso3='AZE' WHERE nombre='Azerbaiyán' AND iso2 IS NULL;
UPDATE paises SET iso2='BS', iso3='BHS' WHERE nombre='Bahamas' AND iso2 IS NULL;
UPDATE paises SET iso2='BD', iso3='BGD' WHERE nombre='Bangladés' AND iso2 IS NULL;
UPDATE paises SET iso2='BB', iso3='BRB' WHERE nombre='Barbados' AND iso2 IS NULL;
UPDATE paises SET iso2='BH', iso3='BHR' WHERE nombre='Baréin' AND iso2 IS NULL;
UPDATE paises SET iso2='BE', iso3='BEL' WHERE nombre='Bélgica' AND iso2 IS NULL;
UPDATE paises SET iso2='BZ', iso3='BLZ' WHERE nombre='Belice' AND iso2 IS NULL;
UPDATE paises SET iso2='BJ', iso3='BEN' WHERE nombre='Benín' AND iso2 IS NULL;
UPDATE paises SET iso2='BY', iso3='BLR' WHERE nombre='Bielorrusia' AND iso2 IS NULL;
UPDATE paises SET iso2='MM', iso3='MMR' WHERE nombre='Birmania' AND iso2 IS NULL;
UPDATE paises SET iso2='BO', iso3='BOL' WHERE nombre='Bolivia' AND iso2 IS NULL;
UPDATE paises SET iso2='BA', iso3='BIH' WHERE nombre='Bosnia y Herzegovina' AND iso2 IS NULL;
UPDATE paises SET iso2='BW', iso3='BWA' WHERE nombre='Botsuana' AND iso2 IS NULL;
UPDATE paises SET iso2='BR', iso3='BRA' WHERE nombre='Brasil' AND iso2 IS NULL;
UPDATE paises SET iso2='BN', iso3='BRN' WHERE nombre='Brunéi' AND iso2 IS NULL;
UPDATE paises SET iso2='BG', iso3='BGR' WHERE nombre='Bulgaria' AND iso2 IS NULL;
UPDATE paises SET iso2='BF', iso3='BFA' WHERE nombre='Burkina Faso' AND iso2 IS NULL;
UPDATE paises SET iso2='BI', iso3='BDI' WHERE nombre='Burundi' AND iso2 IS NULL;
UPDATE paises SET iso2='BT', iso3='BTN' WHERE nombre='Bután' AND iso2 IS NULL;
UPDATE paises SET iso2='CV', iso3='CPV' WHERE nombre='Cabo Verde' AND iso2 IS NULL;
UPDATE paises SET iso2='KH', iso3='KHM' WHERE nombre='Camboya' AND iso2 IS NULL;
UPDATE paises SET iso2='CM', iso3='CMR' WHERE nombre='Camerún' AND iso2 IS NULL;
UPDATE paises SET iso2='CA', iso3='CAN' WHERE nombre='Canadá' AND iso2 IS NULL;
UPDATE paises SET iso2='QA', iso3='QAT' WHERE nombre='Catar' AND iso2 IS NULL;
UPDATE paises SET iso2='TD', iso3='TCD' WHERE nombre='Chad' AND iso2 IS NULL;
UPDATE paises SET iso2='CL', iso3='CHL' WHERE nombre='Chile' AND iso2 IS NULL;
UPDATE paises SET iso2='CN', iso3='CHN' WHERE nombre='China' AND iso2 IS NULL;
UPDATE paises SET iso2='CY', iso3='CYP' WHERE nombre='Chipre' AND iso2 IS NULL;
UPDATE paises SET iso2='CO', iso3='COL' WHERE nombre='Colombia' AND iso2 IS NULL;
UPDATE paises SET iso2='KM', iso3='COM' WHERE nombre='Comoras' AND iso2 IS NULL;
UPDATE paises SET iso2='KP', iso3='PRK' WHERE nombre='Corea del Norte' AND iso2 IS NULL;
UPDATE paises SET iso2='KR', iso3='KOR' WHERE nombre='Corea del Sur' AND iso2 IS NULL;
UPDATE paises SET iso2='CI', iso3='CIV' WHERE nombre='Costa de Marfil' AND iso2 IS NULL;
UPDATE paises SET iso2='CR', iso3='CRI' WHERE nombre='Costa Rica' AND iso2 IS NULL;
UPDATE paises SET iso2='HR', iso3='HRV' WHERE nombre='Croacia' AND iso2 IS NULL;
UPDATE paises SET iso2='CU', iso3='CUB' WHERE nombre='Cuba' AND iso2 IS NULL;
UPDATE paises SET iso2='DK', iso3='DNK' WHERE nombre='Dinamarca' AND iso2 IS NULL;
UPDATE paises SET iso2='DM', iso3='DMA' WHERE nombre='Dominica' AND iso2 IS NULL;
UPDATE paises SET iso2='EC', iso3='ECU' WHERE nombre='Ecuador' AND iso2 IS NULL;
UPDATE paises SET iso2='EG', iso3='EGY' WHERE nombre='Egipto' AND iso2 IS NULL;
UPDATE paises SET iso2='SV', iso3='SLV' WHERE nombre='El Salvador' AND iso2 IS NULL;
UPDATE paises SET iso2='AE', iso3='ARE' WHERE nombre='Emiratos Árabes Unidos' AND iso2 IS NULL;
UPDATE paises SET iso2='ER', iso3='ERI' WHERE nombre='Eritrea' AND iso2 IS NULL;
UPDATE paises SET iso2='SK', iso3='SVK' WHERE nombre='Eslovaquia' AND iso2 IS NULL;
UPDATE paises SET iso2='SI', iso3='SVN' WHERE nombre='Eslovenia' AND iso2 IS NULL;
UPDATE paises SET iso2='ES', iso3='ESP' WHERE nombre='España' AND iso2 IS NULL;
UPDATE paises SET iso2='US', iso3='USA' WHERE nombre='Estados Unidos' AND iso2 IS NULL;
UPDATE paises SET iso2='EE', iso3='EST' WHERE nombre='Estonia' AND iso2 IS NULL;
UPDATE paises SET iso2='SZ', iso3='SWZ' WHERE nombre='Esuatini' AND iso2 IS NULL;
UPDATE paises SET iso2='ET', iso3='ETH' WHERE nombre='Etiopía' AND iso2 IS NULL;
UPDATE paises SET iso2='PH', iso3='PHL' WHERE nombre='Filipinas' AND iso2 IS NULL;
UPDATE paises SET iso2='FI', iso3='FIN' WHERE nombre='Finlandia' AND iso2 IS NULL;
UPDATE paises SET iso2='FJ', iso3='FJI' WHERE nombre='Fiyi' AND iso2 IS NULL;
UPDATE paises SET iso2='FR', iso3='FRA' WHERE nombre='Francia' AND iso2 IS NULL;
UPDATE paises SET iso2='GA', iso3='GAB' WHERE nombre='Gabón' AND iso2 IS NULL;
UPDATE paises SET iso2='GM', iso3='GMB' WHERE nombre='Gambia' AND iso2 IS NULL;
UPDATE paises SET iso2='GE', iso3='GEO' WHERE nombre='Georgia' AND iso2 IS NULL;
UPDATE paises SET iso2='GH', iso3='GHA' WHERE nombre='Ghana' AND iso2 IS NULL;
UPDATE paises SET iso2='GD', iso3='GRD' WHERE nombre='Granada' AND iso2 IS NULL;
UPDATE paises SET iso2='GR', iso3='GRC' WHERE nombre='Grecia' AND iso2 IS NULL;
UPDATE paises SET iso2='GT', iso3='GTM' WHERE nombre='Guatemala' AND iso2 IS NULL;
UPDATE paises SET iso2='GN', iso3='GIN' WHERE nombre='Guinea' AND iso2 IS NULL;
UPDATE paises SET iso2='GW', iso3='GNB' WHERE nombre='Guinea-Bisáu' AND iso2 IS NULL;
UPDATE paises SET iso2='GQ', iso3='GNQ' WHERE nombre='Guinea Ecuatorial' AND iso2 IS NULL;
UPDATE paises SET iso2='GY', iso3='GUY' WHERE nombre='Guyana' AND iso2 IS NULL;
UPDATE paises SET iso2='HT', iso3='HTI' WHERE nombre='Haití' AND iso2 IS NULL;
UPDATE paises SET iso2='HN', iso3='HND' WHERE nombre='Honduras' AND iso2 IS NULL;
UPDATE paises SET iso2='HU', iso3='HUN' WHERE nombre='Hungría' AND iso2 IS NULL;
UPDATE paises SET iso2='IN', iso3='IND' WHERE nombre='India' AND iso2 IS NULL;
UPDATE paises SET iso2='ID', iso3='IDN' WHERE nombre='Indonesia' AND iso2 IS NULL;
UPDATE paises SET iso2='IQ', iso3='IRQ' WHERE nombre='Irak' AND iso2 IS NULL;
UPDATE paises SET iso2='IR', iso3='IRN' WHERE nombre='Irán' AND iso2 IS NULL;
UPDATE paises SET iso2='IE', iso3='IRL' WHERE nombre='Irlanda' AND iso2 IS NULL;
UPDATE paises SET iso2='IS', iso3='ISL' WHERE nombre='Islandia' AND iso2 IS NULL;
UPDATE paises SET iso2='MH', iso3='MHL' WHERE nombre='Islas Marshall' AND iso2 IS NULL;
UPDATE paises SET iso2='SB', iso3='SLB' WHERE nombre='Islas Salomón' AND iso2 IS NULL;
UPDATE paises SET iso2='IL', iso3='ISR' WHERE nombre='Israel' AND iso2 IS NULL;
UPDATE paises SET iso2='IT', iso3='ITA' WHERE nombre='Italia' AND iso2 IS NULL;
UPDATE paises SET iso2='JM', iso3='JAM' WHERE nombre='Jamaica' AND iso2 IS NULL;
UPDATE paises SET iso2='JP', iso3='JPN' WHERE nombre='Japón' AND iso2 IS NULL;
UPDATE paises SET iso2='JO', iso3='JOR' WHERE nombre='Jordania' AND iso2 IS NULL;
UPDATE paises SET iso2='KZ', iso3='KAZ' WHERE nombre='Kazajistán' AND iso2 IS NULL;
UPDATE paises SET iso2='KE', iso3='KEN' WHERE nombre='Kenia' AND iso2 IS NULL;
UPDATE paises SET iso2='KG', iso3='KGZ' WHERE nombre='Kirguistán' AND iso2 IS NULL;
UPDATE paises SET iso2='KI', iso3='KIR' WHERE nombre='Kiribati' AND iso2 IS NULL;
UPDATE paises SET iso2='KW', iso3='KWT' WHERE nombre='Kuwait' AND iso2 IS NULL;
UPDATE paises SET iso2='LA', iso3='LAO' WHERE nombre='Laos' AND iso2 IS NULL;
UPDATE paises SET iso2='LS', iso3='LSO' WHERE nombre='Lesoto' AND iso2 IS NULL;
UPDATE paises SET iso2='LV', iso3='LVA' WHERE nombre='Letonia' AND iso2 IS NULL;
UPDATE paises SET iso2='LB', iso3='LBN' WHERE nombre='Líbano' AND iso2 IS NULL;
UPDATE paises SET iso2='LR', iso3='LBR' WHERE nombre='Liberia' AND iso2 IS NULL;
UPDATE paises SET iso2='LY', iso3='LBY' WHERE nombre='Libia' AND iso2 IS NULL;
UPDATE paises SET iso2='LI', iso3='LIE' WHERE nombre='Liechtenstein' AND iso2 IS NULL;
UPDATE paises SET iso2='LT', iso3='LTU' WHERE nombre='Lituania' AND iso2 IS NULL;
UPDATE paises SET iso2='LU', iso3='LUX' WHERE nombre='Luxemburgo' AND iso2 IS NULL;
UPDATE paises SET iso2='MK', iso3='MKD' WHERE nombre='Macedonia del Norte' AND iso2 IS NULL;
UPDATE paises SET iso2='MG', iso3='MDG' WHERE nombre='Madagascar' AND iso2 IS NULL;
UPDATE paises SET iso2='MY', iso3='MYS' WHERE nombre='Malasia' AND iso2 IS NULL;
UPDATE paises SET iso2='MW', iso3='MWI' WHERE nombre='Malaui' AND iso2 IS NULL;
UPDATE paises SET iso2='MV', iso3='MDV' WHERE nombre='Maldivas' AND iso2 IS NULL;
UPDATE paises SET iso2='ML', iso3='MLI' WHERE nombre='Malí' AND iso2 IS NULL;
UPDATE paises SET iso2='MT', iso3='MLT' WHERE nombre='Malta' AND iso2 IS NULL;
UPDATE paises SET iso2='MA', iso3='MAR' WHERE nombre='Marruecos' AND iso2 IS NULL;
UPDATE paises SET iso2='MU', iso3='MUS' WHERE nombre='Mauricio' AND iso2 IS NULL;
UPDATE paises SET iso2='MR', iso3='MRT' WHERE nombre='Mauritania' AND iso2 IS NULL;
UPDATE paises SET iso2='MX', iso3='MEX' WHERE nombre='México' AND iso2 IS NULL;
UPDATE paises SET iso2='FM', iso3='FSM' WHERE nombre='Micronesia' AND iso2 IS NULL;
UPDATE paises SET iso2='MD', iso3='MDA' WHERE nombre='Moldavia' AND iso2 IS NULL;
UPDATE paises SET iso2='MC', iso3='MCO' WHERE nombre='Mónaco' AND iso2 IS NULL;
UPDATE paises SET iso2='MN', iso3='MNG' WHERE nombre='Mongolia' AND iso2 IS NULL;
UPDATE paises SET iso2='ME', iso3='MNE' WHERE nombre='Montenegro' AND iso2 IS NULL;
UPDATE paises SET iso2='MZ', iso3='MOZ' WHERE nombre='Mozambique' AND iso2 IS NULL;
UPDATE paises SET iso2='NA', iso3='NAM' WHERE nombre='Namibia' AND iso2 IS NULL;
UPDATE paises SET iso2='NR', iso3='NRU' WHERE nombre='Nauru' AND iso2 IS NULL;
UPDATE paises SET iso2='NP', iso3='NPL' WHERE nombre='Nepal' AND iso2 IS NULL;
UPDATE paises SET iso2='NI', iso3='NIC' WHERE nombre='Nicaragua' AND iso2 IS NULL;
UPDATE paises SET iso2='NE', iso3='NER' WHERE nombre='Níger' AND iso2 IS NULL;
UPDATE paises SET iso2='NG', iso3='NGA' WHERE nombre='Nigeria' AND iso2 IS NULL;
UPDATE paises SET iso2='NO', iso3='NOR' WHERE nombre='Noruega' AND iso2 IS NULL;
UPDATE paises SET iso2='NZ', iso3='NZL' WHERE nombre='Nueva Zelanda' AND iso2 IS NULL;
UPDATE paises SET iso2='OM', iso3='OMN' WHERE nombre='Omán' AND iso2 IS NULL;
UPDATE paises SET iso2='NL', iso3='NLD' WHERE nombre='Países Bajos' AND iso2 IS NULL;
UPDATE paises SET iso2='PK', iso3='PAK' WHERE nombre='Pakistán' AND iso2 IS NULL;
UPDATE paises SET iso2='PW', iso3='PLW' WHERE nombre='Palaos' AND iso2 IS NULL;
UPDATE paises SET iso2='PS', iso3='PSE' WHERE nombre='Palestina' AND iso2 IS NULL;
UPDATE paises SET iso2='PA', iso3='PAN' WHERE nombre='Panamá' AND iso2 IS NULL;
UPDATE paises SET iso2='PG', iso3='PNG' WHERE nombre='Papúa Nueva Guinea' AND iso2 IS NULL;
UPDATE paises SET iso2='PY', iso3='PRY' WHERE nombre='Paraguay' AND iso2 IS NULL;
UPDATE paises SET iso2='PE', iso3='PER' WHERE nombre='Perú' AND iso2 IS NULL;
UPDATE paises SET iso2='PL', iso3='POL' WHERE nombre='Polonia' AND iso2 IS NULL;
UPDATE paises SET iso2='PT', iso3='PRT' WHERE nombre='Portugal' AND iso2 IS NULL;
UPDATE paises SET iso2='GB', iso3='GBR' WHERE nombre='Reino Unido' AND iso2 IS NULL;
UPDATE paises SET iso2='CF', iso3='CAF' WHERE nombre='República Centroafricana' AND iso2 IS NULL;
UPDATE paises SET iso2='CZ', iso3='CZE' WHERE nombre='República Checa' AND iso2 IS NULL;
UPDATE paises SET iso2='CG', iso3='COG' WHERE nombre='República del Congo' AND iso2 IS NULL;
UPDATE paises SET iso2='CD', iso3='COD' WHERE nombre='República Democrática del Congo' AND iso2 IS NULL;
UPDATE paises SET iso2='DO', iso3='DOM' WHERE nombre='República Dominicana' AND iso2 IS NULL;
UPDATE paises SET iso2='RW', iso3='RWA' WHERE nombre='Ruanda' AND iso2 IS NULL;
UPDATE paises SET iso2='RO', iso3='ROU' WHERE nombre='Rumanía' AND iso2 IS NULL;
UPDATE paises SET iso2='RU', iso3='RUS' WHERE nombre='Rusia' AND iso2 IS NULL;
UPDATE paises SET iso2='WS', iso3='WSM' WHERE nombre='Samoa' AND iso2 IS NULL;
UPDATE paises SET iso2='KN', iso3='KNA' WHERE nombre='San Cristóbal y Nieves' AND iso2 IS NULL;
UPDATE paises SET iso2='SM', iso3='SMR' WHERE nombre='San Marino' AND iso2 IS NULL;
UPDATE paises SET iso2='VC', iso3='VCT' WHERE nombre='San Vicente y las Granadinas' AND iso2 IS NULL;
UPDATE paises SET iso2='LC', iso3='LCA' WHERE nombre='Santa Lucía' AND iso2 IS NULL;
UPDATE paises SET iso2='ST', iso3='STP' WHERE nombre='Santo Tomé y Príncipe' AND iso2 IS NULL;
UPDATE paises SET iso2='SN', iso3='SEN' WHERE nombre='Senegal' AND iso2 IS NULL;
UPDATE paises SET iso2='RS', iso3='SRB' WHERE nombre='Serbia' AND iso2 IS NULL;
UPDATE paises SET iso2='SC', iso3='SYC' WHERE nombre='Seychelles' AND iso2 IS NULL;
UPDATE paises SET iso2='SL', iso3='SLE' WHERE nombre='Sierra Leona' AND iso2 IS NULL;
UPDATE paises SET iso2='SG', iso3='SGP' WHERE nombre='Singapur' AND iso2 IS NULL;
UPDATE paises SET iso2='SY', iso3='SYR' WHERE nombre='Siria' AND iso2 IS NULL;
UPDATE paises SET iso2='SO', iso3='SOM' WHERE nombre='Somalia' AND iso2 IS NULL;
UPDATE paises SET iso2='LK', iso3='LKA' WHERE nombre='Sri Lanka' AND iso2 IS NULL;
UPDATE paises SET iso2='ZA', iso3='ZAF' WHERE nombre='Sudáfrica' AND iso2 IS NULL;
UPDATE paises SET iso2='SD', iso3='SDN' WHERE nombre='Sudán' AND iso2 IS NULL;
UPDATE paises SET iso2='SS', iso3='SSD' WHERE nombre='Sudán del Sur' AND iso2 IS NULL;
UPDATE paises SET iso2='SE', iso3='SWE' WHERE nombre='Suecia' AND iso2 IS NULL;
UPDATE paises SET iso2='CH', iso3='CHE' WHERE nombre='Suiza' AND iso2 IS NULL;
UPDATE paises SET iso2='SR', iso3='SUR' WHERE nombre='Surinam' AND iso2 IS NULL;
UPDATE paises SET iso2='TH', iso3='THA' WHERE nombre='Tailandia' AND iso2 IS NULL;
UPDATE paises SET iso2='TZ', iso3='TZA' WHERE nombre='Tanzania' AND iso2 IS NULL;
UPDATE paises SET iso2='TJ', iso3='TJK' WHERE nombre='Tayikistán' AND iso2 IS NULL;
UPDATE paises SET iso2='TL', iso3='TLS' WHERE nombre='Timor Oriental' AND iso2 IS NULL;
UPDATE paises SET iso2='TG', iso3='TGO' WHERE nombre='Togo' AND iso2 IS NULL;
UPDATE paises SET iso2='TO', iso3='TON' WHERE nombre='Tonga' AND iso2 IS NULL;
UPDATE paises SET iso2='TT', iso3='TTO' WHERE nombre='Trinidad y Tobago' AND iso2 IS NULL;
UPDATE paises SET iso2='TN', iso3='TUN' WHERE nombre='Túnez' AND iso2 IS NULL;
UPDATE paises SET iso2='TM', iso3='TKM' WHERE nombre='Turkmenistán' AND iso2 IS NULL;
UPDATE paises SET iso2='TR', iso3='TUR' WHERE nombre='Turquía' AND iso2 IS NULL;
UPDATE paises SET iso2='TV', iso3='TUV' WHERE nombre='Tuvalu' AND iso2 IS NULL;
UPDATE paises SET iso2='UA', iso3='UKR' WHERE nombre='Ucrania' AND iso2 IS NULL;
UPDATE paises SET iso2='UG', iso3='UGA' WHERE nombre='Uganda' AND iso2 IS NULL;
UPDATE paises SET iso2='UY', iso3='URY' WHERE nombre='Uruguay' AND iso2 IS NULL;
UPDATE paises SET iso2='UZ', iso3='UZB' WHERE nombre='Uzbekistán' AND iso2 IS NULL;
UPDATE paises SET iso2='VU', iso3='VUT' WHERE nombre='Vanuatu' AND iso2 IS NULL;
UPDATE paises SET iso2='VA', iso3='VAT' WHERE nombre='Vaticano' AND iso2 IS NULL;
UPDATE paises SET iso2='VE', iso3='VEN' WHERE nombre='Venezuela' AND iso2 IS NULL;
UPDATE paises SET iso2='VN', iso3='VNM' WHERE nombre='Vietnam' AND iso2 IS NULL;
UPDATE paises SET iso2='YE', iso3='YEM' WHERE nombre='Yemen' AND iso2 IS NULL;
UPDATE paises SET iso2='DJ', iso3='DJI' WHERE nombre='Yibuti' AND iso2 IS NULL;
UPDATE paises SET iso2='ZM', iso3='ZMB' WHERE nombre='Zambia' AND iso2 IS NULL;
UPDATE paises SET iso2='ZW', iso3='ZWE' WHERE nombre='Zimbabue' AND iso2 IS NULL;

-- ===== Índices útiles =====
CREATE INDEX IF NOT EXISTS idx_continentes_code ON continentes(code);
CREATE INDEX IF NOT EXISTS idx_categorias_slug ON categorias(slug);
CREATE INDEX IF NOT EXISTS idx_paises_iso2 ON paises(iso2);
CREATE INDEX IF NOT EXISTS idx_paises_iso3 ON paises(iso3);

