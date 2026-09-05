import { Button } from "antd";
import { User, Unlink, RefreshCw } from "lucide-react";
import type { Profile } from "../../api/client";

export interface HolderSelectedCardProps {
  profile: Profile;
  onChange: () => void;
  onUnlink: () => void;
  selectedTitle: string;
  changeText: string;
  unlinkText: string;
  autoFilledNotice?: string;
}

export function HolderSelectedCard({
  profile,
  onChange,
  onUnlink,
  selectedTitle,
  changeText,
  unlinkText,
  autoFilledNotice,
}: HolderSelectedCardProps) {
  const addressCity = profile.address?.city;
  const addressState = profile.address?.state;
  const location = [addressCity, addressState].filter(Boolean).join(" - ");

  return (
    <div className="holder-selected-card">
      <div className="holder-selected-card__avatar">
        <User size={20} strokeWidth={1.75} />
      </div>
      <div className="holder-selected-card__content">
        <div className="holder-selected-card__header">
          <span className="holder-selected-card__badge">{selectedTitle}</span>
          <strong className="holder-selected-card__name">{profile.full_name}</strong>
        </div>
        <div className="holder-selected-card__meta">
          {profile.cpf ? (
            <span className="holder-selected-card__meta-item">
              <strong>CPF:</strong> {profile.cpf}
            </span>
          ) : null}
          {profile.mobile_phone ? (
            <span className="holder-selected-card__meta-item">
              <strong>Tel:</strong> {profile.mobile_phone}
            </span>
          ) : null}
          {profile.email ? (
            <span className="holder-selected-card__meta-item">{profile.email}</span>
          ) : null}
          {location ? <span className="holder-selected-card__meta-item">{location}</span> : null}
        </div>
        {autoFilledNotice ? (
          <p className="holder-selected-card__notice">{autoFilledNotice}</p>
        ) : null}
      </div>
      <div className="holder-selected-card__actions">
        <Button icon={<RefreshCw size={14} />} onClick={onChange} size="small" type="text">
          {changeText}
        </Button>
        <Button danger icon={<Unlink size={14} />} onClick={onUnlink} size="small" type="text">
          {unlinkText}
        </Button>
      </div>
    </div>
  );
}
