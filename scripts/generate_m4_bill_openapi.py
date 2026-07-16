#!/usr/bin/env python3
import json
from pathlib import Path

path = Path("api/openapi.yaml")
spec = json.loads(path.read_text(encoding="utf-8"))
spec["info"]["version"] = "0.6.0"
security = [{"sessionCookie": []}]
ref = lambda name: {"$ref": f"#/components/schemas/{name}"}
resp_ref = lambda name: {"$ref": f"#/components/responses/{name}"}
content = lambda schema: {"application/json": {"schema": schema}}
body = lambda schema: {"required": True, "content": content(schema)}
response = lambda description, schema=None: {"description": description, **({"content": content(schema)} if schema else {})}
uuid_param = lambda name: {"name": name, "in": "path", "required": True, "schema": {"type": "string", "format": "uuid"}}
common = {"400": resp_ref("BadRequest"), "401": resp_ref("Unauthorized"), "403": resp_ref("Forbidden"), "503": resp_ref("ServiceUnavailable")}
not_found = {**common, "404": resp_ref("NotFound")}
conflict = {**not_found, "409": resp_ref("Conflict"), "422": resp_ref("ValidationError")}

spec["paths"]["/api/v1/bill-types"] = {
    "get": {"operationId": "listBillTypes", "summary": "List administrable bill types", "tags": ["Bills"], "security": security,
        "parameters": [
            {"name": "limit", "in": "query", "schema": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000, "default": 100}},
            {"name": "offset", "in": "query", "schema": {"type": "integer", "format": "int32", "minimum": 0, "default": 0}},
            {"name": "sort", "in": "query", "schema": ref("BillTypeSortField")},
            {"name": "order", "in": "query", "schema": ref("SortOrder")},
            {"name": "label", "in": "query", "schema": {"type": "string"}},
            {"name": "active", "in": "query", "schema": {"type": "boolean"}},
        ], "responses": {"200": response("Paginated bill type list", ref("BillTypePageResponse")), **common}},
    "post": {"operationId": "createBillType", "summary": "Create an administrable bill type", "tags": ["Bills"], "security": security,
        "requestBody": body(ref("BillTypeValuesRequest")),
        "responses": {"201": response("Created bill type", ref("BillType")), "409": resp_ref("Conflict"), "422": resp_ref("ValidationError"), **common}},
}
spec["paths"]["/api/v1/bill-types/{bill_type_id}"] = {
    "parameters": [uuid_param("bill_type_id")],
    "get": {"operationId": "getBillType", "summary": "Read one bill type", "tags": ["Bills"], "security": security, "responses": {"200": response("Bill type", ref("BillType")), **not_found}},
    "put": {"operationId": "updateBillType", "summary": "Replace bill type settings using optimistic concurrency", "tags": ["Bills"], "security": security, "requestBody": body(ref("UpdateBillTypeRequest")), "responses": {"200": response("Updated bill type", ref("BillType")), **conflict}},
    "delete": {"operationId": "deleteBillType", "summary": "Permanently delete an unused bill type", "tags": ["Bills"], "security": security, "requestBody": body(ref("DeleteBillResourceRequest")), "responses": {"204": response("Bill type permanently deleted"), **not_found, "409": resp_ref("Conflict")}},
}
spec["paths"]["/api/v1/bills"] = {
    "get": {"operationId": "listBills", "summary": "List Profile-owned bills", "tags": ["Bills"], "security": security,
        "parameters": [
            {"name": "limit", "in": "query", "schema": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000, "default": 100}},
            {"name": "offset", "in": "query", "schema": {"type": "integer", "format": "int32", "minimum": 0, "default": 0}},
            {"name": "sort", "in": "query", "schema": ref("BillSortField")},
            {"name": "order", "in": "query", "schema": ref("SortOrder")},
            {"name": "owner_profile_id", "in": "query", "schema": {"type": "string", "format": "uuid"}},
            {"name": "bill_type_id", "in": "query", "schema": {"type": "string", "format": "uuid"}},
            {"name": "reference", "in": "query", "schema": {"type": "string"}},
            {"name": "competence", "in": "query", "schema": {"type": "string", "pattern": "^[0-9]{4}-(0[1-9]|1[0-2])$"}},
            {"name": "record_state", "in": "query", "schema": ref("BillRecordState")},
            {"name": "status", "in": "query", "schema": ref("BillStatus")},
            {"name": "holder_profile_id", "in": "query", "schema": {"type": "string", "format": "uuid"}},
        ], "responses": {"200": response("Paginated bill list", ref("BillPageResponse")), **common}},
    "post": {"operationId": "createBill", "summary": "Create a Profile-owned bill", "tags": ["Bills"], "security": security, "requestBody": body(ref("BillValuesRequest")), "responses": {"201": response("Created bill", ref("Bill")), **conflict}},
}
spec["paths"]["/api/v1/bills/{bill_id}"] = {
    "parameters": [uuid_param("bill_id")],
    "get": {"operationId": "getBill", "summary": "Read one bill", "tags": ["Bills"], "security": security, "responses": {"200": response("Bill", ref("Bill")), **not_found}},
    "put": {"operationId": "updateBill", "summary": "Replace bill values using optimistic concurrency", "tags": ["Bills"], "security": security, "requestBody": body(ref("UpdateBillRequest")), "responses": {"200": response("Updated bill", ref("Bill")), **conflict}},
    "delete": {"operationId": "deleteBill", "summary": "Permanently delete an available bill", "tags": ["Bills"], "security": security, "requestBody": body(ref("DeleteBillResourceRequest")), "responses": {"204": response("Bill permanently deleted"), **not_found, "409": resp_ref("Conflict")}},
}
spec["paths"]["/api/v1/bills/{bill_id}/duplicate"] = {
    "post": {"operationId": "duplicateBill", "summary": "Duplicate a bill into an independent record", "tags": ["Bills"], "security": security, "parameters": [uuid_param("bill_id")], "responses": {"201": response("Duplicated bill", ref("Bill")), **not_found, "409": resp_ref("Conflict")}}
}
spec["paths"]["/api/v1/bills/{bill_id}/current-use"] = {
    "put": {"operationId": "assignBillCurrentUse", "summary": "Assign or replace the current bill holder when supported", "tags": ["Bills"], "security": security, "parameters": [uuid_param("bill_id")], "requestBody": body(ref("AssignBillCurrentUseRequest")), "responses": {"200": response("Current bill use", ref("BillCurrentUse")), **not_found, "409": resp_ref("Conflict")}},
    "delete": {"operationId": "returnBillCurrentUse", "summary": "Return a bill to available status", "tags": ["Bills"], "security": security, "parameters": [uuid_param("bill_id")], "responses": {"204": response("Bill returned"), **not_found, "409": resp_ref("Conflict")}},
}

schemas = spec["components"]["schemas"]
schemas["BillTypeSortField"] = {"type": "string", "enum": ["label", "technical_key", "created_at", "updated_at"], "default": "label"}
schemas["BillSortField"] = {"type": "string", "enum": ["reference_value", "type_label", "competence", "amount", "created_at", "updated_at"], "default": "reference_value"}
schemas["BillRecordState"] = {"type": "string", "enum": ["CURRENT", "REPLACED", "EXPIRED", "ARCHIVED"]}
schemas["BillStatus"] = {"type": "string", "enum": ["AVAILABLE", "IN_USE"]}
schemas["BillTypeValuesRequest"] = {"type": "object", "additionalProperties": False, "required": ["technical_key", "label", "active", "supports_current_use"], "properties": {
    "technical_key": {"type": "string", "minLength": 2, "maxLength": 64, "pattern": "^[a-z][a-z0-9_]{1,63}$"}, "label": {"type": "string", "minLength": 1, "maxLength": 120}, "active": {"type": "boolean"}, "supports_current_use": {"type": "boolean"}}}
schemas["UpdateBillTypeRequest"] = {"type": "object", "additionalProperties": False, "required": ["technical_key", "label", "active", "supports_current_use", "version"], "properties": {**schemas["BillTypeValuesRequest"]["properties"], "version": {"type": "integer", "format": "int64", "minimum": 1}}}
schemas["BillType"] = {"type": "object", "additionalProperties": False, "required": ["id", "technical_key", "label", "active", "supports_current_use", "version", "created_at", "updated_at"], "properties": {
    "id": {"type": "string", "format": "uuid"}, "technical_key": {"type": "string"}, "label": {"type": "string"}, "active": {"type": "boolean"}, "supports_current_use": {"type": "boolean"}, "version": {"type": "integer", "format": "int64", "minimum": 1}, "created_at": {"type": "string", "format": "date-time"}, "updated_at": {"type": "string", "format": "date-time"}}}
value_properties = {
    "owner_profile_id": {"type": "string", "format": "uuid"}, "bill_type_id": {"type": "string", "format": "uuid"},
    "printed_holder_name": {"type": "string", "maxLength": 200}, "printed_address": {"type": "string", "maxLength": 500},
    "reference_value": {"type": "string", "maxLength": 500}, "competence": {"type": "string", "pattern": "^$|^[0-9]{4}-(0[1-9]|1[0-2])$"},
    "amount": {"type": "string", "pattern": "^$|^[0-9]+(?:\\.[0-9]{1,2})?$"}, "currency": {"type": "string", "pattern": "^$|^[A-Z]{3}$"},
    "notes": {"type": "string", "maxLength": 5000}, "record_state": ref("BillRecordState")}
value_required = list(value_properties)
schemas["BillValuesRequest"] = {"type": "object", "additionalProperties": False, "required": value_required, "properties": value_properties}
schemas["UpdateBillRequest"] = {"type": "object", "additionalProperties": False, "required": value_required + ["version"], "properties": {**value_properties, "version": {"type": "integer", "format": "int64", "minimum": 1}}}
schemas["DeleteBillResourceRequest"] = {"type": "object", "additionalProperties": False, "required": ["version", "confirmation"], "properties": {"version": {"type": "integer", "format": "int64", "minimum": 1}, "confirmation": {"type": "string", "const": "Confirmar"}}}
schemas["AssignBillCurrentUseRequest"] = {"type": "object", "additionalProperties": False, "required": ["holder_profile_id"], "properties": {"holder_profile_id": {"type": "string", "format": "uuid"}}}
schemas["BillCurrentUse"] = {"type": "object", "additionalProperties": False, "required": ["holder_profile_id", "assigned_at"], "properties": {"holder_profile_id": {"type": "string", "format": "uuid"}, "assigned_at": {"type": "string", "format": "date-time"}}}
schemas["Bill"] = {"type": "object", "additionalProperties": False, "required": ["id"] + value_required + ["status", "type", "current_use", "version", "created_at", "updated_at"], "properties": {
    "id": {"type": "string", "format": "uuid"}, **value_properties, "status": ref("BillStatus"), "type": ref("BillType"),
    "current_use": {"oneOf": [ref("BillCurrentUse"), {"type": "null"}]}, "version": {"type": "integer", "format": "int64", "minimum": 1},
    "created_at": {"type": "string", "format": "date-time"}, "updated_at": {"type": "string", "format": "date-time"}}}
schemas["BillPageMeta"] = {"type": "object", "additionalProperties": False, "required": ["total", "limit", "offset", "sort_field", "sort_order"], "properties": {
    "total": {"type": "integer", "format": "int64", "minimum": 0}, "limit": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000},
    "offset": {"type": "integer", "format": "int32", "minimum": 0}, "sort_field": ref("BillSortField"), "sort_order": ref("SortOrder")}}
schemas["BillTypePageResponse"] = {"type": "object", "additionalProperties": False, "required": ["types", "page"], "properties": {"types": {"type": "array", "items": ref("BillType")}, "page": {"type": "object", "additionalProperties": False, "required": ["total", "limit", "offset", "sort_field", "sort_order"], "properties": {"total": {"type": "integer", "format": "int64", "minimum": 0}, "limit": {"type": "integer", "format": "int32", "minimum": 1, "maximum": 1000}, "offset": {"type": "integer", "format": "int32", "minimum": 0}, "sort_field": ref("BillTypeSortField"), "sort_order": ref("SortOrder")}}}}
schemas["BillPageResponse"] = {"type": "object", "additionalProperties": False, "required": ["bills", "page"], "properties": {"bills": {"type": "array", "items": ref("Bill")}, "page": ref("BillPageMeta")}}
path.write_text(json.dumps(spec, ensure_ascii=False, separators=(",", ":")) + "\n", encoding="utf-8")
