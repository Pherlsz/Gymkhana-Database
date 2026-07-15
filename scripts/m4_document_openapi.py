#!/usr/bin/env python3
import json
import sys
from pathlib import Path


def ref(name: str) -> dict:
    return {"$ref": f"#/components/schemas/{name}"}


def response(name: str) -> dict:
    return {"$ref": f"#/components/responses/{name}"}


def parameter(name: str, location: str, schema: dict, required: bool = False) -> dict:
    value = {"name": name, "in": location, "schema": schema}
    if required:
        value["required"] = True
    return value


def request_body(schema_name: str) -> dict:
    return {
        "required": True,
        "content": {"application/json": {"schema": ref(schema_name)}},
    }


def json_response(description: str, schema_name: str) -> dict:
    return {
        "description": description,
        "content": {"application/json": {"schema": ref(schema_name)}},
    }


def secured(operation: dict) -> dict:
    operation["security"] = [{"sessionCookie": []}]
    return operation


def standard_errors(*names: str) -> dict:
    status = {
        "BadRequest": "400",
        "Unauthorized": "401",
        "Forbidden": "403",
        "NotFound": "404",
        "Conflict": "409",
        "ValidationError": "422",
        "ServiceUnavailable": "503",
    }
    return {status[name]: response(name) for name in names}


def values_properties() -> dict:
    return {
        "owner_profile_id": {"type": "string", "format": "uuid"},
        "document_type_id": {"type": "string", "format": "uuid"},
        "identifier_value": {"type": "string", "minLength": 1, "maxLength": 500},
        "document_date": {"type": "string", "maxLength": 10},
        "notes": {"type": "string", "maxLength": 5000},
        "record_state": ref("DocumentRecordState"),
    }


def type_values_properties() -> dict:
    return {
        "technical_key": {"type": "string", "minLength": 2, "maxLength": 64},
        "label": {"type": "string", "minLength": 1, "maxLength": 120},
        "active": {"type": "boolean"},
        "uniqueness_policy": ref("DocumentUniquenessPolicy"),
        "validation_regex": {"type": "string", "maxLength": 500},
        "date_required": {"type": "boolean"},
    }


def main() -> None:
    path = Path(sys.argv[1] if len(sys.argv) > 1 else "api/openapi.yaml")
    document = json.loads(path.read_text(encoding="utf-8"))
    document["info"]["version"] = "0.5.0"
    schemas = document["components"]["schemas"]

    schemas.update(
        {
            "DocumentUniquenessPolicy": {
                "type": "string",
                "enum": ["NONE", "PER_PROFILE", "GLOBAL_BY_TYPE"],
            },
            "DocumentRecordState": {
                "type": "string",
                "enum": ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"],
            },
            "DocumentStatus": {
                "type": "string",
                "enum": ["AVAILABLE", "IN_USE"],
            },
            "DocumentTypeSortField": {
                "type": "string",
                "enum": ["label", "technical_key", "created_at", "updated_at"],
                "default": "label",
            },
            "DocumentSortField": {
                "type": "string",
                "enum": ["identifier_value", "type_label", "document_date", "created_at", "updated_at"],
                "default": "identifier_value",
            },
            "DocumentTypeValuesRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": list(type_values_properties()),
                "properties": type_values_properties(),
            },
            "UpdateDocumentTypeRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": [*type_values_properties(), "version"],
                "properties": {
                    **type_values_properties(),
                    "version": {"type": "integer", "format": "int64", "minimum": 1},
                },
            },
            "DeleteDocumentResourceRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": ["version", "confirmation"],
                "properties": {
                    "version": {"type": "integer", "format": "int64", "minimum": 1},
                    "confirmation": {"type": "string", "const": "Confirmar"},
                },
            },
            "DocumentType": {
                "type": "object",
                "additionalProperties": False,
                "required": [
                    "id",
                    *type_values_properties(),
                    "version",
                    "created_at",
                    "updated_at",
                ],
                "properties": {
                    "id": {"type": "string", "format": "uuid"},
                    **type_values_properties(),
                    "version": {"type": "integer", "format": "int64", "minimum": 1},
                    "created_at": {"type": "string", "format": "date-time"},
                    "updated_at": {"type": "string", "format": "date-time"},
                },
            },
            "DocumentValuesRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": list(values_properties()),
                "properties": values_properties(),
            },
            "UpdateDocumentRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": [*values_properties(), "version"],
                "properties": {
                    **values_properties(),
                    "version": {"type": "integer", "format": "int64", "minimum": 1},
                },
            },
            "AssignDocumentCurrentUseRequest": {
                "type": "object",
                "additionalProperties": False,
                "required": ["holder_profile_id"],
                "properties": {"holder_profile_id": {"type": "string", "format": "uuid"}},
            },
            "DocumentCurrentUse": {
                "type": "object",
                "additionalProperties": False,
                "required": ["holder_profile_id", "assigned_at"],
                "properties": {
                    "holder_profile_id": {"type": "string", "format": "uuid"},
                    "assigned_at": {"type": "string", "format": "date-time"},
                },
            },
            "Document": {
                "type": "object",
                "additionalProperties": False,
                "required": [
                    "id",
                    *values_properties(),
                    "status",
                    "type",
                    "current_use",
                    "version",
                    "created_at",
                    "updated_at",
                ],
                "properties": {
                    "id": {"type": "string", "format": "uuid"},
                    **values_properties(),
                    "status": ref("DocumentStatus"),
                    "type": ref("DocumentType"),
                    "current_use": {**ref("DocumentCurrentUse"), "nullable": True},
                    "version": {"type": "integer", "format": "int64", "minimum": 1},
                    "created_at": {"type": "string", "format": "date-time"},
                    "updated_at": {"type": "string", "format": "date-time"},
                },
            },
            "DocumentTypePageMeta": {
                "type": "object",
                "additionalProperties": False,
                "required": ["total", "limit", "offset", "sort_field", "sort_order"],
                "properties": {
                    "total": {"type": "integer", "format": "int64", "minimum": 0},
                    "limit": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000},
                    "offset": {"type": "integer", "format": "int32", "minimum": 0},
                    "sort_field": ref("DocumentTypeSortField"),
                    "sort_order": ref("SortOrder"),
                },
            },
            "DocumentPageMeta": {
                "type": "object",
                "additionalProperties": False,
                "required": ["total", "limit", "offset", "sort_field", "sort_order"],
                "properties": {
                    "total": {"type": "integer", "format": "int64", "minimum": 0},
                    "limit": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000},
                    "offset": {"type": "integer", "format": "int32", "minimum": 0},
                    "sort_field": ref("DocumentSortField"),
                    "sort_order": ref("SortOrder"),
                },
            },
            "DocumentTypePageResponse": {
                "type": "object",
                "additionalProperties": False,
                "required": ["types", "page"],
                "properties": {
                    "types": {"type": "array", "items": ref("DocumentType")},
                    "page": ref("DocumentTypePageMeta"),
                },
            },
            "DocumentPageResponse": {
                "type": "object",
                "additionalProperties": False,
                "required": ["documents", "page"],
                "properties": {
                    "documents": {"type": "array", "items": ref("Document")},
                    "page": ref("DocumentPageMeta"),
                },
            },
        }
    )

    path_document_type_id = parameter(
        "document_type_id", "path", {"type": "string", "format": "uuid"}, True
    )
    path_document_id = parameter(
        "document_id", "path", {"type": "string", "format": "uuid"}, True
    )
    type_list_parameters = [
        parameter("limit", "query", {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000, "default": 100}),
        parameter("offset", "query", {"type": "integer", "format": "int32", "minimum": 0, "default": 0}),
        parameter("sort", "query", ref("DocumentTypeSortField")),
        parameter("order", "query", ref("SortOrder")),
        parameter("label", "query", {"type": "string"}),
        parameter("active", "query", {"type": "boolean"}),
    ]
    document_list_parameters = [
        parameter("limit", "query", {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000, "default": 100}),
        parameter("offset", "query", {"type": "integer", "format": "int32", "minimum": 0, "default": 0}),
        parameter("sort", "query", ref("DocumentSortField")),
        parameter("order", "query", ref("SortOrder")),
        parameter("owner_profile_id", "query", {"type": "string", "format": "uuid"}),
        parameter("document_type_id", "query", {"type": "string", "format": "uuid"}),
        parameter("identifier", "query", {"type": "string"}),
        parameter("record_state", "query", ref("DocumentRecordState")),
        parameter("status", "query", ref("DocumentStatus")),
        parameter("holder_profile_id", "query", {"type": "string", "format": "uuid"}),
    ]

    document["paths"].update(
        {
            "/api/v1/document-types": {
                "get": secured(
                    {
                        "operationId": "listDocumentTypes",
                        "summary": "List administrable document types",
                        "tags": ["Documents"],
                        "parameters": type_list_parameters,
                        "responses": {
                            "200": json_response("Paginated document type list", "DocumentTypePageResponse"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "ServiceUnavailable"),
                        },
                    }
                ),
                "post": secured(
                    {
                        "operationId": "createDocumentType",
                        "summary": "Create an administrable document type",
                        "tags": ["Documents"],
                        "requestBody": request_body("DocumentTypeValuesRequest"),
                        "responses": {
                            "201": json_response("Created document type", "DocumentType"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "Conflict", "ValidationError", "ServiceUnavailable"),
                        },
                    }
                ),
            },
            "/api/v1/document-types/{document_type_id}": {
                "parameters": [path_document_type_id],
                "get": secured(
                    {
                        "operationId": "getDocumentType",
                        "summary": "Read one document type",
                        "tags": ["Documents"],
                        "responses": {
                            "200": json_response("Document type", "DocumentType"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "ServiceUnavailable"),
                        },
                    }
                ),
                "put": secured(
                    {
                        "operationId": "updateDocumentType",
                        "summary": "Replace document type settings using optimistic concurrency",
                        "tags": ["Documents"],
                        "requestBody": request_body("UpdateDocumentTypeRequest"),
                        "responses": {
                            "200": json_response("Updated document type", "DocumentType"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ValidationError", "ServiceUnavailable"),
                        },
                    }
                ),
                "delete": secured(
                    {
                        "operationId": "deleteDocumentType",
                        "summary": "Permanently delete an unused document type",
                        "tags": ["Documents"],
                        "requestBody": request_body("DeleteDocumentResourceRequest"),
                        "responses": {
                            "204": {"description": "Document type permanently deleted"},
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ServiceUnavailable"),
                        },
                    }
                ),
            },
            "/api/v1/documents": {
                "get": secured(
                    {
                        "operationId": "listDocuments",
                        "summary": "List Profile-owned documents",
                        "tags": ["Documents"],
                        "parameters": document_list_parameters,
                        "responses": {
                            "200": json_response("Paginated document list", "DocumentPageResponse"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "ServiceUnavailable"),
                        },
                    }
                ),
                "post": secured(
                    {
                        "operationId": "createDocument",
                        "summary": "Create a Profile-owned document",
                        "tags": ["Documents"],
                        "requestBody": request_body("DocumentValuesRequest"),
                        "responses": {
                            "201": json_response("Created document", "Document"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ValidationError", "ServiceUnavailable"),
                        },
                    }
                ),
            },
            "/api/v1/documents/{document_id}": {
                "parameters": [path_document_id],
                "get": secured(
                    {
                        "operationId": "getDocument",
                        "summary": "Read one document",
                        "tags": ["Documents"],
                        "responses": {
                            "200": json_response("Document", "Document"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "ServiceUnavailable"),
                        },
                    }
                ),
                "put": secured(
                    {
                        "operationId": "updateDocument",
                        "summary": "Replace document values using optimistic concurrency",
                        "tags": ["Documents"],
                        "requestBody": request_body("UpdateDocumentRequest"),
                        "responses": {
                            "200": json_response("Updated document", "Document"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ValidationError", "ServiceUnavailable"),
                        },
                    }
                ),
                "delete": secured(
                    {
                        "operationId": "deleteDocument",
                        "summary": "Permanently delete an available document",
                        "tags": ["Documents"],
                        "requestBody": request_body("DeleteDocumentResourceRequest"),
                        "responses": {
                            "204": {"description": "Document permanently deleted"},
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ServiceUnavailable"),
                        },
                    }
                ),
            },
            "/api/v1/documents/{document_id}/duplicate": {
                "post": secured(
                    {
                        "operationId": "duplicateDocument",
                        "summary": "Duplicate a document into an independent record",
                        "tags": ["Documents"],
                        "parameters": [path_document_id],
                        "responses": {
                            "201": json_response("Duplicated document", "Document"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ServiceUnavailable"),
                        },
                    }
                )
            },
            "/api/v1/documents/{document_id}/current-use": {
                "put": secured(
                    {
                        "operationId": "assignDocumentCurrentUse",
                        "summary": "Assign or replace the current document holder",
                        "tags": ["Documents"],
                        "parameters": [path_document_id],
                        "requestBody": request_body("AssignDocumentCurrentUseRequest"),
                        "responses": {
                            "200": json_response("Assigned current use", "DocumentCurrentUse"),
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ServiceUnavailable"),
                        },
                    }
                ),
                "delete": secured(
                    {
                        "operationId": "returnDocumentCurrentUse",
                        "summary": "Return a document and remove its current-use relation",
                        "tags": ["Documents"],
                        "parameters": [path_document_id],
                        "responses": {
                            "204": {"description": "Current use returned"},
                            **standard_errors("BadRequest", "Unauthorized", "Forbidden", "NotFound", "Conflict", "ServiceUnavailable"),
                        },
                    }
                ),
            },
        }
    )

    path.write_text(json.dumps(document, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
