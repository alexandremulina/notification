# Modelo de Microsserviço

Este é um modelo para criar microsserviços usando Go, Gin e PostgreSQL.

## Pré-requisitos

-   Go 1.16+
-   PostgreSQL
-   [golang-migrate](https://github.com/golang-migrate/migrate)
-   [sqlc](https://sqlc.dev/)
-   [swag](https://github.com/swaggo/swag)
-   Docker e Docker Compose (opcional)

### Executando localmente

4. Execute os seguintes comandos:

    ```
    # Executar migrações do banco de dados
    make migrate

    # Iniciar a aplicação
    make run
    ```

    Alternativamente, você pode executar todos os comandos de uma vez:

    ```
    make all
    ```

### Executando com Docker

4. Execute os seguintes comandos:

    ```
    # Construir e iniciar os contêineres Docker
    make docker-all

    # Para parar os contêineres
    make docker-down
    ```

A aplicação estará disponível em `http://localhost:8080`.

## Documentação da API

A documentação Swagger está disponível em `http://localhost:8080/swagger/index.html`.

## Gerando Documentação Swagger

Para regenerar a documentação Swagger após fazer alterações na API, execute:
