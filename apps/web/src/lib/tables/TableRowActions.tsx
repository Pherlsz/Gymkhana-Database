import { Button, Dropdown, Modal, type MenuProps } from "antd";
import { MoreHorizontal } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { useLayoutEffect, useMemo, useState } from "react";
import { ICON, ICON_STROKE } from "../../components/icons";
import { t, useI18n } from "../../i18n";
import { queryKeys } from "../api/queryKeys";
import { returnBillCurrentUse, returnDocumentCurrentUse } from "../api/client";
import { CurrentUseControls, type CurrentUseRecord } from "../records/RecordEditorCommon";
import {
  tableRowActionSubject,
  tableRowAllowsCurrentUse,
  tableRowEntityId,
  tableRowEntityKind,
  tableRowInUse,
  type TableRowEntityKind,
} from "./tableRowNavigation";
import type { TableRow } from "./tableRows";

export function TableRowActions({
  row,
  section,
  isResult,
  onEdit,
  onNotice,
}: {
  row: TableRow;
  section: "profile" | "documents" | "bills";
  isResult: boolean;
  onEdit: (row: TableRow) => void;
  onNotice: (message: string) => void;
}) {
  const { messages } = useI18n();
  const queryClient = useQueryClient();
  const copy = messages.tables.rowActions;
  const panel = messages.records.panel;
  const kind = tableRowEntityKind(row, { section, isResult });
  const subject = tableRowActionSubject(row, kind) || copy.untitled;
  const allowsCurrentUse = tableRowAllowsCurrentUse(row, kind);
  const inUse = tableRowInUse(row);
  const [menuMounted, setMenuMounted] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [useOpen, setUseOpen] = useState(false);

  useLayoutEffect(() => {
    if (menuMounted) setMenuOpen(true);
  }, [menuMounted]);

  const items = useMemo(() => {
    const next: NonNullable<MenuProps["items"]> = [
      { key: "edit", label: messages.common.actions.edit },
    ];
    if (!allowsCurrentUse) return next;
    if (inUse) {
      next.push(
        { key: "substitute", label: panel.substituteHolder },
        { key: "return", label: panel.returnUse },
      );
      return next;
    }
    next.push({ key: "assign", label: copy.assignUse });
    return next;
  }, [
    allowsCurrentUse,
    copy.assignUse,
    inUse,
    messages.common.actions.edit,
    panel.returnUse,
    panel.substituteHolder,
  ]);

  const currentUseRecord = currentUseFromRow(row, tableRowEntityId(row, isResult));

  const refresh = async (entity: TableRowEntityKind) => {
    if (entity === "document") {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tables.documents() });
      await queryClient.invalidateQueries({ queryKey: queryKeys.home.documentsInUse });
      return;
    }
    if (entity === "bill") {
      await queryClient.invalidateQueries({ queryKey: queryKeys.tables.bills() });
      await queryClient.invalidateQueries({ queryKey: queryKeys.home.billsInUse });
    }
  };

  const closeMenu = () => {
    setMenuOpen(false);
    setMenuMounted(false);
  };

  const confirmReturn = () => {
    if (kind !== "document" && kind !== "bill") return;
    const id = tableRowEntityId(row, isResult);
    Modal.confirm({
      title: copy.returnConfirmTitle,
      content: copy.returnConfirmBody,
      okText: panel.returnUse,
      cancelText: messages.common.actions.cancel,
      onOk: async () => {
        if (kind === "document") await returnDocumentCurrentUse(id);
        else await returnBillCurrentUse(id);
        await refresh(kind);
        onNotice(panel.currentUseReturnedNotice);
      },
    });
  };

  const trigger = (
    <Button
      aria-label={t(copy.menu, { label: subject })}
      className="spreadsheet-table__row-actions-trigger"
      size="small"
      type="text"
      onClick={(event) => {
        event.stopPropagation();
        if (!menuMounted) setMenuMounted(true);
      }}
      onKeyDown={(event) => event.stopPropagation()}
    >
      <MoreHorizontal aria-hidden size={ICON.md} strokeWidth={ICON_STROKE} />
    </Button>
  );

  return (
    <>
      {menuMounted ? (
        <Dropdown
          autoAdjustOverflow={false}
          destroyOnHidden
          getPopupContainer={() => document.body}
          menu={{
            items,
            onClick: ({ key, domEvent }) => {
              domEvent.stopPropagation();
              closeMenu();
              if (key === "edit") {
                onEdit(row);
                return;
              }
              if (key === "assign" || key === "substitute") {
                setUseOpen(true);
                return;
              }
              if (key === "return") confirmReturn();
            },
          }}
          open={menuOpen}
          rootClassName="spreadsheet-table__row-actions-menu"
          trigger={["click"]}
          onOpenChange={(open) => {
            setMenuOpen(open);
            if (!open) setMenuMounted(false);
          }}
        >
          {trigger}
        </Dropdown>
      ) : (
        trigger
      )}
      {currentUseRecord && (kind === "document" || kind === "bill") ? (
        <Modal
          destroyOnHidden
          footer={null}
          open={useOpen}
          title={panel.currentUseTitle}
          onCancel={() => setUseOpen(false)}
        >
          <CurrentUseControls
            framed={false}
            kind={kind}
            record={currentUseRecord}
            onChanged={async (message) => {
              await refresh(kind);
              onNotice(message);
              setUseOpen(false);
            }}
          />
        </Modal>
      ) : null}
    </>
  );
}

function currentUseFromRow(row: TableRow, id: string): CurrentUseRecord | undefined {
  const status = row.cells.status_key;
  if (status !== "AVAILABLE" && status !== "IN_USE") return undefined;
  const holderId = String(row.cells.current_holder_id ?? "").trim();
  const holderName = String(row.cells.current_holder ?? "").trim();
  return {
    id,
    status,
    current_use: holderId ? { holder_profile_id: holderId, holder_full_name: holderName } : null,
  };
}
