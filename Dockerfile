# Ein Abbild für den Cluster. Zwei Stufen, und die zweite ist fast leer.
#
# Das Programm ist ein einzelnes Binär ohne CGO: Vorlagen, Mittel, Migrationen
# und Schriften stecken über embed.FS darin. Also braucht die Laufzeitstufe
# keine Distribution, keine Bibliotheken und keine Paketverwaltung — nur die
# Wurzelzertifikate für den Mailversand und einen Benutzer, der nicht root ist.
# Beides bringt distroless/static mit.
#
# Gebaut wird für linux/amd64, wie der Rest des Projekts seit 1.4.

FROM golang:1.26 AS bau

WORKDIR /src

# Erst die Modulliste, dann der Quelltext: so bleibt die Schicht mit den
# Abhängigkeiten im Zwischenspeicher, solange go.mod und go.sum sich nicht
# ändern.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Dieselben Angaben, die der Freigabe-Ablauf ins Binär schreibt. Ohne sie meldet
# `holzcloud version` "dev", und die Verwaltung zeigt in ihrer Fusszeile eine
# Fassung, die niemand einer Freigabe zuordnen kann — was nach AGPL §13 genau
# die Angabe ist, die gebraucht wird.
ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /holzcloud ./cmd/holzcloud

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=bau /holzcloud /holzcloud

# Das Datenverzeichnis wird zur Laufzeit eingehängt und hier bewusst nicht als
# VOLUME angelegt: ein VOLUME ohne Einhängepunkt erzeugt bei jedem Start
# stillschweigend ein namenloses Volume, und die SQLite-Datei darin wäre beim
# nächsten Start weg, ohne dass irgendwo etwas fehlgeschlagen wäre.
ENV HOLZCLOUD_DATA_DIR=/data \
    HOLZCLOUD_PORT=8080

EXPOSE 8080

# 65532, der Benutzer aus distroless. Das eingehängte Verzeichnis muss ihm
# gehören — im Cluster über fsGroup.
USER nonroot:nonroot

ENTRYPOINT ["/holzcloud"]
CMD ["serve"]
