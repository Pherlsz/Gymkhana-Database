import type { LucideIcon } from "lucide-react";
import {
  Baby,
  BookUser,
  Briefcase,
  Bus,
  Car,
  ClipboardList,
  Coins,
  CreditCard,
  Droplet,
  Dumbbell,
  FileText,
  GraduationCap,
  HardHat,
  Heart,
  HeartHandshake,
  HeartPulse,
  IdCard,
  Plane,
  Scale,
  ScrollText,
  ShieldCheck,
  Smile,
  Stethoscope,
  Ticket,
  User,
  Vote,
  Wifi,
  Zap,
} from "lucide-react";
import { normalizeCatalogToken, type HomeCatalogItem } from "./catalogTaxonomy";
import type { AppCardVariant } from "../../components/AppCard";

export function groupCategoryIcon(key: string): LucideIcon {
  switch (key) {
    case "personal":
      return User;
    case "bills":
      return Zap;
    case "identity":
      return ShieldCheck;
    case "work":
      return Briefcase;
    case "socialHealth":
      return HeartPulse;
    case "education":
      return GraduationCap;
    case "certificates":
      return ScrollText;
    default:
      return ClipboardList;
  }
}

export function groupCategoryTheme(key: string): AppCardVariant {
  switch (key) {
    case "bills":
      return "bills";
    case "identity":
      return "identity";
    case "work":
      return "work";
    case "socialHealth":
      return "health";
    case "education":
      return "education";
    case "certificates":
      return "certificates";
    default:
      return "other";
  }
}

export function catalogSubIcon(item: HomeCatalogItem): LucideIcon {
  if (item.kind === "people") return User;
  const token = `${normalizeCatalogToken(item.technicalKey)} ${normalizeCatalogToken(item.label)}`;

  if (item.kind === "bill" || token.includes("luz") || token.includes("energia")) {
    if (token.includes("agua")) return Droplet;
    if (token.includes("internet") || token.includes("wifi")) return Wifi;
    return Zap;
  }

  if (token.includes("cnh") || token.includes("habilitacao")) return Car;
  if (token.includes("passaporte")) return Plane;
  if (token.includes("rg") || token.includes("identidade")) return IdCard;

  if (
    token.includes("ctps") ||
    token.includes("carteira de trabalho") ||
    token.includes("trabalho")
  ) {
    return BookUser;
  }
  if (token.includes("pis") || token.includes("pasep")) return Coins;
  if (token.includes("crea")) return HardHat;
  if (token.includes("oab")) return Scale;
  if (token.includes("crm")) return Stethoscope;
  if (token.includes("cro")) return Smile;
  if (token.includes("coren")) return HeartPulse;

  if (token.includes("titulo")) return Vote;
  if (token.includes("sus")) return Heart;
  if (token.includes("cidadao")) return CreditCard;

  if (token.includes("estudant") || token.includes("escolar")) return GraduationCap;

  if (token.includes("nascimento")) return Baby;
  if (token.includes("casamento")) return HeartHandshake;

  if (token.includes("cref")) return Dumbbell;
  if (token.includes("teu")) return Bus;
  if (token.includes("tri")) return Ticket;

  return FileText;
}
