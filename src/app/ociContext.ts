export type OCIContextSelection = {
  profileId: string;
  region: string;
};

const STORAGE_KEY = "oci-lifecycle:selected-context";
export const OCI_CONTEXT_EVENT = "oci-context-change";

export function getSelectedOCIContext(): OCIContextSelection {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return { profileId: "", region: "" };
    const parsed = JSON.parse(raw) as Partial<OCIContextSelection>;
    return {
      profileId: String(parsed.profileId || ""),
      region: String(parsed.region || "")
    };
  } catch {
    return { profileId: "", region: "" };
  }
}

export function setSelectedOCIContext(selection: OCIContextSelection) {
  const next = {
    profileId: selection.profileId.trim(),
    region: selection.region.trim()
  };
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
  window.dispatchEvent(new CustomEvent<OCIContextSelection>(OCI_CONTEXT_EVENT, { detail: next }));
  return next;
}

export function onOCIContextChange(callback: (selection: OCIContextSelection) => void) {
  const handler = (event: Event) => {
    callback((event as CustomEvent<OCIContextSelection>).detail ?? getSelectedOCIContext());
  };
  window.addEventListener(OCI_CONTEXT_EVENT, handler);
  return () => window.removeEventListener(OCI_CONTEXT_EVENT, handler);
}
