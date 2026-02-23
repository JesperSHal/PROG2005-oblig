# CountryInfo REST API (Assignment 1)

## Overview

This project implements a RESTful web service in Go that provides country-related information and currency exchange rates for neighbouring countries. The service integrates two external APIs: a self-hosted instance of the REST Countries API and a self-hosted Currency API. The purpose of the service is not to replicate external data, but to dynamically interrogate third-party services and recombine their information into value-added responses in real time.

The implementation strictly uses the Go standard library. All routing, HTTP communication, JSON handling, validation, and error management are implemented manually without third-party dependencies.

When running locally, the service is available at:

http://localhost:8080/countryinfo/v1/

---

## Endpoints

The service exposes three resource root paths as defined in the assignment specification.

The diagnostics endpoint (`/countryinfo/v1/status/`) provides a runtime overview of dependent services. It probes the REST Countries API and the Currency API and reports their HTTP status codes. In addition, it returns the API version and the uptime of the service in seconds since startup. The endpoint returns HTTP 200 if both dependent services respond successfully; otherwise, it returns an appropriate error status (typically 502).

The country information endpoint (`/countryinfo/v1/info/{two_letter_country_code}`) returns general information about a country identified by its ISO 3166-2 two-letter code (for example, `/countryinfo/v1/info/no`). The response includes the country name, continents, population, area, languages, neighbouring country codes, flag URL, and capital. Input is validated before any external request is made. If the ISO code format is invalid, the service returns 400. If the country cannot be found, 404 is returned. Failures from upstream services are mapped to 502.

The exchange endpoint (`/countryinfo/v1/exchange/{two_letter_country_code}`) returns currency exchange rates from the base currency of the input country to the currencies of its neighbouring countries. The workflow is as follows: the service retrieves the input country from the REST Countries API, extracts its neighbouring countries and base currency, retrieves exchange rates from the Currency API, and filters the rates to include only those corresponding to neighbouring countries. The response format follows the updated simplified specification, returning a single map of currency codes to exchange rate values. Proper validation and error handling are applied at each stage of the request pipeline.

---

## Architectural Approach

The service follows a layered request flow using the Go standard library. Incoming HTTP requests are handled using `net/http` and routed via `http.ServeMux`. JSON encoding and decoding are handled through `encoding/json`. A shared HTTP client with timeout is used to protect the service from hanging upstream calls.

The architecture distinguishes clearly between upstream models (representing data returned by third-party APIs) and client-facing response models. This separation ensures that the service does not expose external data structures directly and remains robust to potential upstream changes.

To improve efficiency and minimize external load, the exchange endpoint retrieves currency rates only once per request and filters them locally rather than performing multiple currency lookups.

---

## Error Handling and Validation

All endpoints validate input before invoking external services. The service differentiates between client errors (400), not-found cases (404), and upstream failures (502). JSON-formatted error responses are returned consistently to maintain API clarity.

Special care is taken when parsing REST Countries responses, as some endpoints may return either an object or an array depending on the query. The implementation handles both cases defensively.

The service also protects against external service delays by using request timeouts. This ensures stability and predictable behavior even if upstream APIs become slow or temporarily unavailable.

---

## Deployment

The service is designed to be deployed on Render. Development is performed locally, and the deployment process builds directly from a private GitHub repository. The application reads the `PORT` environment variable to support cloud deployment environments.

The deployed service URL must remain active at the time of submission, as required by the assignment instructions.

---

## Running the Service Locally

The application can be started locally using:

```bash
go run .
```

After startup, the endpoints can be tested using a browser, Postman, or curl. For example:
```bash
curl http://localhost:8080/countryinfo/v1/info/no
```
## Deployment Process
The development process was incremental. Initial work focused on establishing routing and HTTP handling. The Currency API was integrated first to understand external API interrogation and JSON decoding. The REST Countries API was then integrated, followed by the composition logic required for the exchange endpoint. Finally, error handling and response structure were aligned precisely with the assignment specification.

An AI assistant was used during development strictly as a learning assistant and conceptual guide. It was used to clarify aspects of Go’s standard library, API integration strategies, and architectural decisions. No code was directly generated or copied from AI tools; all implementation was written and structured manually by me. Since I had never coded with Go before, i needed some extra guidance, therefore AI was used to help me understand GO concepts.

## External Dependencies
The service depends on the following self-hosted APIs provided for the course:

REST Countries API: http://129.241.150.113:8080/v3.1/
Currency API: http://129.241.150.113:9090/currency/

These services are treated as external black-box dependencies and are interrogated dynamically at runtime.