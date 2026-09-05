import { DatePicker, Input } from "antd";
import dayjs from "dayjs";

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
  copy: {
    fieldFatherName: string;
    fieldFatherBirthDate: string;
    fieldMotherName: string;
    fieldMotherBirthDate: string;
    fieldWeddingDate: string;
    fieldParentsWeddingDate: string;
    placeholderDate: string;
  };
}

export function PersonFamilyGroup({ state, onChange, disabled, copy }: PersonFamilyGroupProps) {
  const isDisabled = Boolean(disabled);

  return (
    <div className="cadastro-grid">
      {/* Pai */}
      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-father">
            {copy.fieldFatherName}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-fam-father"
            placeholder={copy.fieldFatherName}
            value={state.fatherName}
            onChange={(e) => onChange({ fatherName: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-father-bdate">
            {copy.fieldFatherBirthDate}
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

      {/* Mãe */}
      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-mother">
            {copy.fieldMotherName}
          </label>
          <Input
            disabled={isDisabled}
            id="cad-fam-mother"
            placeholder={copy.fieldMotherName}
            value={state.motherName}
            onChange={(e) => onChange({ motherName: e.target.value })}
          />
        </div>
      </div>

      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-mother-bdate">
            {copy.fieldMotherBirthDate}
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

      {/* Casamentos */}
      <div className="cadastro-col-6">
        <div className="cadastro-field">
          <label className="cadastro-field__label" htmlFor="cad-fam-wedding">
            {copy.fieldWeddingDate}
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
            {copy.fieldParentsWeddingDate}
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
