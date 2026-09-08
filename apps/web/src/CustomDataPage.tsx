import { Flex, Layout, Typography } from "antd";
import { useState } from "react";
import "./customdata.css";
import { CustomEntitiesAdmin } from "./CustomEntitiesAdmin";
import { CustomEntityTypesAdmin } from "./CustomEntityTypesAdmin";
import { CustomFieldsAdmin } from "./CustomFieldsAdmin";
import { useApplicationSession } from "./session";

type Section = "types" | "fields" | "entities";

export function CustomDataPage() {
  const session = useApplicationSession();
  const [section, setSection] = useState<Section>("fields");
  const canAdminister = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";
  return (
    <Layout className="page-measure">
      <header className="page-header">
        <Typography.Title level={2} className="page-title">
          Campos extras
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          Defina campos tipados reutilizáveis e entidades vinculadas. Os valores aparecem na grade,
          não nesta tela.
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.25rem">
          <nav aria-label="Definições de campos extras" className="custom-data-tabs">
            <button
              className={section === "entities" ? "custom-data-tabs__active" : undefined}
              onClick={() => setSection("entities")}
              type="button"
            >
              Entidades
            </button>
            {canAdminister ? (
              <>
                <button
                  className={section === "types" ? "custom-data-tabs__active" : undefined}
                  onClick={() => setSection("types")}
                  type="button"
                >
                  Tipos
                </button>
                <button
                  className={section === "fields" ? "custom-data-tabs__active" : undefined}
                  onClick={() => setSection("fields")}
                  type="button"
                >
                  Campos e opções
                </button>
              </>
            ) : null}
          </nav>
          {section === "types" && canAdminister ? <CustomEntityTypesAdmin /> : null}
          {section === "fields" && canAdminister ? <CustomFieldsAdmin /> : null}
          {section === "entities" ? <CustomEntitiesAdmin /> : null}
        </Flex>
      </div>
    </Layout>
  );
}
