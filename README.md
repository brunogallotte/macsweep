# macsweep

Recupera espaço no seu Mac e, antes disso, responde a pergunta que ninguém
consegue responder: para onde foram os GB.

Feito para Mac de desenvolvedor, onde o disco quase nunca está cheio de mídia.
Está cheio de `node_modules`, `.next`, caches de toolchain e dados de apps
Electron, todos regeneráveis, todos invisíveis no Finder.

Digitar `macsweep` abre um app que você **permanece dentro**: escolhe um
módulo, ele varre, você marca, ele remove, mostra o ganho e volta ao menu com
os números do disco atualizados.

```
  macsweep
  ──────────────────────────────────────────────────────────
  12.2 GB livres de 228 GB
  ██████████████████████████████████████████████████████░░░░
  ! 37.8 GB na Lixeira esperando para ser esvaziado

  ▸ Aplicativos          desinstala programas, do mais pesado ao mais leve
    Caches               ferramentas de dev, apps e sistema
    Projetos             artefatos de build regeneráveis nos repositórios
    Analisar disco       mapa navegável de onde foram os GB
    Esvaziar a Lixeira   libera 37.8 GB de verdade
    Sair

  ↑↓ navegar   enter abrir   r atualizar   q sair
```

Cada módulo também roda direto por subcomando, com `--dry-run` e `--json`,
para script e automação. Fora de um terminal a CLI imprime o resumo e sai,
então nada quebra num pipe.

## Instalação

```sh
brew install brunogallotte/tap/macsweep
```

Ou, sem Homebrew:

```sh
curl -fsSL https://raw.githubusercontent.com/brunogallotte/macsweep/main/install.sh | sh
```

Binário universal de 5 MB, sem runtime, sem dependência. Apple Silicon e Intel.

Se o `brew install` reclamar que suas Command Line Tools estão desatualizadas,
é uma exigência do próprio Homebrew para qualquer fórmula, não do macsweep.
Atualize pelos Ajustes do sistema, ou use o instalador via `curl` acima, que
não depende de nada disso.

## Os quatro comandos

### `macsweep apps`

Lista os aplicativos ordenados pelo espaço que ocupam **de verdade**: o bundle
somado aos arquivos que ele espalhou por `~/Library`. Marque vários, confira a
lista completa de caminhos e desinstale de uma vez.

```
app                            total    bundle  resíduos  origem
Docker                      10.73 GB   2.10 GB   8.64 GB  arrastado
JetBrains Rider              5.71 GB   5.71 GB   0.00 GB  arrastado
Microsoft Teams              2.12 GB   1.09 GB   1.04 GB  arrastado  (rodando)
Meld                         0.29 GB   0.29 GB   0.00 GB  Homebrew
```

Reconhece a origem de cada app e age de acordo: cask do Homebrew é delegado
para `brew uninstall --cask`, que tem a lista curada do que remover; app da App
Store é sinalizado; app do Setapp é recusado, porque quem gerencia é o cliente
do Setapp; app em execução é bloqueado até você fechá-lo, porque matar um app
com trabalho não salvo seria uma perda de dados causada pela ferramenta.

### `macsweep clean`

Catálogo de caches conhecidos, medidos e explicados. Cada item mostra **como
ele volta**, que é a diferença entre marcar uma caixa com confiança e fechar a
ferramenta com medo.

Cobre nuget, npm, pnpm, yarn, bun, cargo, rustup, maven, gradle, pip, uv,
poetry, asdf, mise, CocoaPods, Homebrew, Xcode (DerivedData, DeviceSupport,
Archives, simuladores), caches de apps Electron e Chromium, logs, estado de
janelas e o cache temporário por usuário em `/var/folders`, que quase nenhuma
ferramenta olha.

Para Docker, Go e Homebrew, chama a própria ferramenta (`docker system prune`,
`go clean -modcache`, `brew cleanup`), porque só ela sabe recuperar o espaço
dela corretamente.

O que é puro cache vem marcado. O que custa tempo para reconstruir vem
desmarcado. O que pode fazer falta só aparece com `--risky`.

### `macsweep projects`

Varre seus repositórios e lista o que dá para apagar sem perder nada, agrupado
por projeto, com o comando que regenera cada artefato e há quanto tempo o
projeto está parado.

```
163 projetos, 413 artefatos, 54.65 GB

  11.97 GB  4Manage              ocioso há 1d
             11.52 GB  .angular       ng build
  6.53 GB   Varos-Website        ocioso há 295d
              4.71 GB  .next          next build
              1.40 GB  node_modules   npm install
```

Duas coisas que as alternativas erram e esta acerta:

- **Procura onde você realmente guarda código.** As ferramentas existentes
  varrem apenas `~/Projects`, `~/Code`, `~/dev`, `~/GitHub` e `~/Workspace`.
  Quem guarda repositório em `~/Documents` não vê nada. Aqui `~/Documents` está
  no padrão.
- **Exige um manifesto ao lado.** Uma pasta chamada `bin`, `dist` ou `build` só
  conta como artefato se houver um `package.json`, `.csproj`, `Cargo.toml` ou
  equivalente junto dela. Sem isso, é só uma pasta, e pode ser seu código.

Projeto ativo aparece desmarcado. Marcado vem só o que está parado há mais de
30 dias, ajustável com `--idle-days`.

### `macsweep empty`

O comando que de fato devolve espaço, e a parte que quase toda ferramenta
dessas esconde: **mover para a Lixeira não libera um byte**. É um rename
dentro do mesmo volume. O espaço volta quando a Lixeira é esvaziada.

Então o macsweep registra o que mandou para lá e esvazia exatamente isso,
medindo o ganho real no disco:

```
  ✓ 388 itens apagados                      37.8 GB

    espaco livre    12.2 GB  →   50.0 GB
    ganho          +37.8 GB   █████████████░░░░░░░
```

O que você jogou fora pela sua conta continua na Lixeira, intacto: o comando
só mexe no que está nos manifestos do macsweep.

### `macsweep analyze`

Mapa navegável do disco, no espírito do `ncdu`, com tamanho real em disco em
todos os níveis.

Colapsa corredores: em vez de repetir os mesmos 8,6 GB em
`Containers` → `com.docker.docker` → `Data` → `vms` → `0` → `data`, mostra
apenas os níveis que carregam informação.

## Segurança

Uma ferramenta que apaga arquivos precisa ser chata a respeito disso.

- **Tudo vai para a Lixeira.** `--delete` existe, mas é preciso pedir. E o
  relatório nunca anuncia um ganho que não houve: mover para a Lixeira não
  libera espaço, e ele diz isso em vez de inventar um número.
- **Toda operação grava um manifesto** com o caminho original de cada item, e
  `macsweep undo` devolve tudo ao lugar. Essa é a rede de segurança que
  realmente segura: o "Colocar de volta" do Finder é quebrado para todos os
  itens depois do primeiro de um lote, um bug aberto na Apple desde 2015.
- **Nada é apagável por padrão.** Só o que está sob uma raiz conhecida, e uma
  regra de negação vence qualquer permissão, por mais específica que ela seja.
  Chaveiro, iCloud Drive, backups de iOS, biblioteca de fotos, chaves SSH e os
  `Documents` dentro de containers de apps nunca são tocados.
- **Correspondência exata por padrão.** Resíduo de app é encontrado pelo
  identificador do bundle, não por um pedaço do nome. O modo por nome existe
  em `--deep`, vem desmarcado e avisado. É exatamente esse atalho que já fez
  um desinstalador apagar a pasta do GarageBand ao remover um app da Logitech.
- **Nunca usa `sudo` sozinho.** O que exige admin é listado à parte, com o
  comando para você rodar.
- **`--dry-run` em tudo**, e `--json` em tudo, para script.

### Números honestos

O tamanho é lido de `st_blocks`, o mesmo que o `du` usa, e não do tamanho
lógico. Sem isso o `Docker.raw` apareceria com 64 GB em vez dos 8,6 GB que
ocupa de fato.

Ainda assim, o previsto e o recuperado são números diferentes no APFS: clones
compartilham blocos e snapshots locais seguram os bytes do que você apagou.
Então o macsweep anuncia o previsto como "até" e mede o resultado real
comparando o espaço livre antes e depois. Quando a diferença é grande, ele diz
o porquê e manda você para o `macsweep doctor`.

### Acesso total ao disco

Sem ele, algumas pastas simplesmente não podem ser lidas e os totais saem por
baixo, sem parecer que saíram. O macsweep conta esses casos durante a varredura
e avisa em vez de apresentar um total incompleto como se fosse completo.

```
$ macsweep doctor
  ! acesso total ao disco: faltando
    3 pastas não puderam ser lidas, então os totais saem por baixo
```

A permissão é concedida ao **terminal**, não ao macsweep, e só vale depois de
fechar e reabrir o terminal por completo.

## Desenvolvimento

```sh
go test ./...                                   # testes
go build -o macsweep ./cmd/macsweep             # binário local
MACSWEEP_TRASH_TEST=1 go test ./internal/trash/ # exercita a Lixeira de verdade
```

Sem cgo. As poucas chamadas de Objective-C que a Lixeira exige passam por
[purego](https://github.com/ebitengine/purego), o que mantém `CGO_ENABLED=0` e
permite gerar as duas arquiteturas a partir de um runner só.

## Licença

MIT
