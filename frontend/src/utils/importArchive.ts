const ARCHIVE_NAME_RE = /\.(zip|gz|json\.gz)$/i

export function isImportArchiveFile(file: File): boolean {
  return ARCHIVE_NAME_RE.test(file.name.trim())
}

export function partitionImportFiles(files: File[]): { archives: File[]; jsonFiles: File[] } {
  const archives: File[] = []
  const jsonFiles: File[] = []
  for (const file of files) {
    if (isImportArchiveFile(file)) {
      archives.push(file)
    } else {
      jsonFiles.push(file)
    }
  }
  return { archives, jsonFiles }
}
