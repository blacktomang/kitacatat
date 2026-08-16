import { useBooks } from "../lib/hooks";

interface BookSelectorProps {
  selected: string | null;
  onChange: (bookId: string | null) => void;
}

export function BookSelector({ selected, onChange }: BookSelectorProps) {
  const { data: books = [] } = useBooks();

  return (
    <label className="flex flex-col text-xs font-medium text-slate-500">
      Buku
      <select
        value={selected ?? "all"}
        onChange={(e) => onChange(e.target.value === "all" ? null : e.target.value)}
        className="mt-1 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-700 shadow-sm focus:border-slate-400 focus:outline-none"
      >
        <option value="all">Semua</option>
        {books.map((book) => (
          <option key={book.id} value={book.id}>
            {book.name}
            {book.role !== "owner" ? " (dibagikan)" : ""}
          </option>
        ))}
      </select>
    </label>
  );
}
