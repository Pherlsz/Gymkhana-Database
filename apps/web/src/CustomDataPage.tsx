import { Flex, Layout, Typography } from "antd";
import { useState } from "react";
import "./customdata.css";
import { useApplicationSession } from "./App";
import { CustomEntitiesAdmin } from "./CustomEntitiesAdmin";
import { CustomEntityTypesAdmin } from "./CustomEntityTypesAdmin";
import { CustomFieldsAdmin } from "./CustomFieldsAdmin";
import { CustomRecordValuesAdmin } from "./CustomRecordValuesAdmin";

type Section = "values" | "types" | "fields" | "entities";

export function CustomDataPage() {
  const session = useApplicationSession();
  const [section, setSection] = useState<Section>("values");
  const canAdminister = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  return (
    <Layout style={{ maxWidth: "lg", margin: "0 auto" }}>
      <header className="page-header">
        <div className="page-eyebrow">M5 · Dados personalizados tipados</div>
        <Typography.Title level={1} className="page-title">
          Dados personalizados
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          Defina campos tipados reutilizáveis e cadastre entidades vinculadas ou independentes sem
          depender de JSON livre.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.25rem">
          <nav aria-label="Seções de dados personalizados" className="custom-data-tabs">
            <button
              className={section === "values" ? "custom-data-tabs__active" : undefined}
              onClick={() => setSection("values")}
            >
              Pessoas e registros
            </button>
            <button
              className={section === "entities" ? "custom-data-tabs__active" : undefined}
              onClick={() => setSection("entities")}
            >
              Entidades
            </button>
            {canAdminister ? (
              <>
                <button
                  className={section === "types" ? "custom-data-tabs__active" : undefined}
                  onClick={() => setSection("types")}
                >
                  Tipos
                </button>
                <button
                  className={section === "fields" ? "custom-data-tabs__active" : undefined}
                  onClick={() => setSection("fields")}
                >
                  Campos e opções
                </button>
              </>
            ) : null}
          </nav>
          {section === "values" ? <CustomRecordValuesAdmin /> : null}
          {section === "types" && canAdminister ? <CustomEntityTypesAdmin /> : null}
          {section === "fields" && canAdminister ? <CustomFieldsAdmin /> : null}
          {section === "entities" ? <CustomEntitiesAdmin /> : null}
        </Flex>
      </div>
    </Layout>
  );
}
