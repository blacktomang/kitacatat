const rupiah = new Intl.NumberFormat("id-ID", {
  style: "currency",
  currency: "IDR",
  maximumFractionDigits: 0,
});

export function formatRupiah(amount: number): string {
  return rupiah.format(amount);
}

const dateFmt = new Intl.DateTimeFormat("id-ID", {
  day: "2-digit",
  month: "short",
  year: "numeric",
});

export function formatDate(iso: string): string {
  return dateFmt.format(new Date(iso));
}

const monthFmt = new Intl.DateTimeFormat("id-ID", {
  month: "short",
  year: "2-digit",
});

export function formatMonth(iso: string): string {
  return monthFmt.format(new Date(iso));
}
