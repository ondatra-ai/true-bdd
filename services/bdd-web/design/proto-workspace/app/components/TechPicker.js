"use client";

import { useRef, useState } from "react";

// Searchable "add technology" combobox: type-to-filter over the catalog,
// mouse click or Enter adds the highlighted match. Deliberately no
// portal/positioning library — the dropdown is just an absolutely
// positioned <ul> under the input, styled from the mockup's own
// square/border-only language.
export default function TechPicker({ catalog, selected, onAdd }) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef(null);

  const available = catalog.filter(
    (item) =>
      !selected.includes(item) &&
      item.toLowerCase().includes(query.trim().toLowerCase())
  );

  function addItem(item) {
    onAdd(item);
    setQuery("");
    setHighlight(0);
    inputRef.current?.focus();
  }

  function handleKeyDown(e) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setOpen(true);
      setHighlight((h) => Math.min(h + 1, available.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (available[highlight]) addItem(available[highlight]);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  }

  return (
    <div className="tech-picker">
      <input
        ref={inputRef}
        type="text"
        placeholder="Add technology…"
        value={query}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onChange={(e) => {
          setQuery(e.target.value);
          setHighlight(0);
          setOpen(true);
        }}
        onKeyDown={handleKeyDown}
        className="tech-picker__input"
        aria-label="Add technology"
        role="combobox"
        aria-expanded={open}
        aria-autocomplete="list"
      />
      {open && available.length > 0 && (
        <ul className="tech-picker__list" role="listbox">
          {available.map((item, i) => (
            <li
              key={item}
              role="option"
              aria-selected={i === highlight}
              className={i === highlight ? "is-highlighted" : ""}
              onMouseDown={(e) => e.preventDefault()}
              onMouseEnter={() => setHighlight(i)}
              onClick={() => addItem(item)}
            >
              {item}
            </li>
          ))}
        </ul>
      )}
      {open && available.length === 0 && (
        <ul className="tech-picker__list">
          <li className="tech-picker__empty" aria-disabled="true">
            No match
          </li>
        </ul>
      )}
    </div>
  );
}
