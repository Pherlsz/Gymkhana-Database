import { Flex, Layout, Segmented, Typography } from "antd";
import { useState } from "react";
import "./customdata.css";
import { CustomEntitiesAdmin } from "./CustomEntitiesAdmin";
import { CustomEntityTypesAdmin } from "./CustomEntityTypesAdmin";
import { CustomFieldsAdmin } from "./CustomFieldsAdmin";
import { useI18n } from "./i18n";
import { useApplicationSession } from "./session";

type Section = "types" | "fields" | "entities";

export function CustomDataPage() {
  const session = useApplicationSession();
  const { messages } = useI18n();
  const copy = messages.admin.customData;
  const [section, setSection] = useState<Section>("fields");
  const canAdminister = session.user.role === "ADMIN" || session.user.role === "SUPERADMIN";

  const options = [
    { label: copy.tabEntities, value: "entities" as const },
    ...(canAdminister
      ? [
          { label: copy.tabTypes, value: "types" as const },
          { label: copy.tabFields, value: "fields" as const },
        ]
      : []),
  ];

  return (
    <Layout className="page-measure">
      <header className="page-header">
        <Typography.Title level={2} className="page-title">
          {copy.title}
        </Typography.Title>
        <Typography.Paragraph className="page-description">
          {copy.description}
        </Typography.Paragraph>
      </header>
      <div className="page-content">
        <Flex vertical gap="1.25rem">
          <nav aria-label={copy.navAriaLabel}>
            <Segmented<Section>
              className="segmented-tabs"
              onChange={(val) => setSection(val)}
              options={options}
              size="middle"
              value={section}
            />
          </nav>
          {section === "types" && canAdminister ? <CustomEntityTypesAdmin /> : null}
          {section === "fields" && canAdminister ? <CustomFieldsAdmin /> : null}
          {section === "entities" ? <CustomEntitiesAdmin /> : null}
        </Flex>
      </div>
    </Layout>
  );
}
