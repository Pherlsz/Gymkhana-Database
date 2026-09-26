import { type ReactNode } from "react";

function inlineMd(text: string): ReactNode {
  const parts: ReactNode[] = [];
  const pattern = /(\*\*[^*]+?\*\*|`[^`]+`|\*[^*]+?\*)/g;
  let last = 0;
  let token = pattern.exec(text);
  let index = 0;
  while (token) {
    if (token.index > last) parts.push(text.slice(last, token.index));
    const value = token[0];
    if (value.startsWith("**")) parts.push(<strong key={index}>{value.slice(2, -2)}</strong>);
    else if (value.startsWith("`")) parts.push(<code key={index}>{value.slice(1, -1)}</code>);
    else parts.push(<em key={index}>{value.slice(1, -1)}</em>);
    last = token.index + value.length;
    index += 1;
    token = pattern.exec(text);
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

function isList(line: string, ordered: boolean): boolean {
  return ordered ? /^\s*\d+\. /.test(line) : /^\s*[-*] /.test(line);
}

function stripList(line: string, ordered: boolean): string {
  return line.replace(ordered ? /^\s*\d+\. / : /^\s*[-*] /, "");
}

export function AssistantMarkdown({ text }: { text: string }) {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let cursor = 0;
  let key = 0;
  while (cursor < lines.length) {
    const line = lines[cursor] ?? "";
    if (line.startsWith("```")) {
      const body: string[] = [];
      cursor += 1;
      while (cursor < lines.length && !(lines[cursor] ?? "").startsWith("```")) {
        body.push(lines[cursor] ?? "");
        cursor += 1;
      }
      cursor += 1;
      blocks.push(
        <pre className="assistant-md__pre" key={key}>
          <code>{body.join("\n")}</code>
        </pre>,
      );
      key += 1;
      continue;
    }
    const heading = /^(#{1,3}) /.exec(line);
    if (heading) {
      const Tag = (heading[1] === "#" ? "h3" : heading[1] === "##" ? "h4" : "h5") as
        | "h3"
        | "h4"
        | "h5";
      blocks.push(
        <Tag className="assistant-md__heading" key={key}>
          {inlineMd(line.slice(heading[0].length))}
        </Tag>,
      );
      key += 1;
      cursor += 1;
      continue;
    }
    const ordered = isList(line, true);
    if (ordered || isList(line, false)) {
      const items: string[] = [];
      while (cursor < lines.length && isList(lines[cursor] ?? "", ordered)) {
        items.push(stripList(lines[cursor] ?? "", ordered));
        cursor += 1;
      }
      const List = ordered ? "ol" : "ul";
      blocks.push(
        <List className="assistant-md__list" key={key}>
          {items.map((item, itemKey) => (
            <li key={itemKey}>{inlineMd(item)}</li>
          ))}
        </List>,
      );
      key += 1;
      continue;
    }
    if (!line.trim()) {
      cursor += 1;
      continue;
    }
    const paragraph = [line];
    cursor += 1;
    while (
      cursor < lines.length &&
      (lines[cursor] ?? "").trim() &&
      !/^(```|#{1,3} |\s*[-*] |\s*\d+\. )/.test(lines[cursor] ?? "")
    ) {
      paragraph.push(lines[cursor] ?? "");
      cursor += 1;
    }
    blocks.push(<p key={key}>{inlineMd(paragraph.join(" "))}</p>);
    key += 1;
  }
  return <>{blocks}</>;
}
