import { DatePicker, Input } from "antd";
import dayjs from "dayjs";
import { useI18n } from "../../../i18n";

export interface PersonFamilyState {
  fatherName: string;
  fatherBirthDate?: string | undefined;
  motherName: string;
  motherBirthDate?: string | undefined;
  weddingDate?: string | undefined;
  parentsWeddingDate?: string | undefined;
}

export interface PersonFamilyGroupProps {
  state: PersonFamilyState;
  onChange: (patch: Partial<PersonFamilyState>) => void;
  disabled?: boolean;
  copy?: Record<string, string>;
}

export function PersonFamilyGroup({
  state,
  onChange,
  disabled,
  copy: customCopy,
}: PersonFamilyGroupProps) {
  const { messages } = useI18n();
  const labels = messages.common.labels;
  const copy = { ...messages.tables.cadastro, ...customCopy };
  const isDisabled = Boolean(disabled);

  return (
    <div className="cadastro-grid">
      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-father">
            {labels.fatherName}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-fam-father"
            placeholder={labels.fatherName}
            value={state.fatherName}
            onChange={(e) => onChange({ fatherName: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-father-bdate">
            {labels.fatherBirthDate}
          </label>
          <DatePicker
            disabled={isDisabled}
            format="DD/MM/YYYY"
            id="cad-fam-father-bdate"
            placeholder={copy.placeholderDate}
            style={{ width: "100%" }}
            value={state.fatherBirthDate ? dayjs(state.fatherBirthDate) : null}
            onChange={(d) => onChange({ fatherBirthDate: d ? d.format("YYYY-MM-DD") : undefined })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-mother">
            {labels.motherName}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-fam-mother"
            placeholder={labels.motherName}
            value={state.motherName}
            onChange={(e) => onChange({ motherName: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-mother-bdate">
            {labels.motherBirthDate}
          </label>
          <DatePicker
            disabled={isDisabled}
            format="DD/MM/YYYY"
            id="cad-fam-mother-bdate"
            placeholder={copy.placeholderDate}
            style={{ width: "100%" }}
            value={state.motherBirthDate ? dayjs(state.motherBirthDate) : null}
            onChange={(d) => onChange({ motherBirthDate: d ? d.format("YYYY-MM-DD") : undefined })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-wedding">
            {labels.weddingDate}
          </label>
          <DatePicker
            disabled={isDisabled}
            format="DD/MM/YYYY"
            id="cad-fam-wedding"
            placeholder={copy.placeholderDate}
            style={{ width: "100%" }}
            value={state.weddingDate ? dayjs(state.weddingDate) : null}
            onChange={(d) => onChange({ weddingDate: d ? d.format("YYYY-MM-DD") : undefined })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-parents-wedding">
            {labels.parentsWeddingDate}
          </label>
          <DatePicker
            disabled={isDisabled}
            format="DD/MM/YYYY"
            id="cad-fam-parents-wedding"
            placeholder={copy.placeholderDate}
            style={{ width: "100%" }}
            value={state.parentsWeddingDate ? dayjs(state.parentsWeddingDate) : null}
            onChange={(d) =>
              onChange({ parentsWeddingDate: d ? d.format("YYYY-MM-DD") : undefined })
            }
          />
        </div>
      </div>
    </div>
  );
}
