import React, {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode,
} from "react";

// Define the shape of the context state
interface AppContextType {
  phoneNumber: string | undefined;
  setPhoneNumber: (value: string) => void;
}

// Create the context with a default value of undefined
const AppContext = createContext<AppContextType | undefined>(undefined);

// Define the props for the provider
interface AppProviderProps {
  children: ReactNode;
}

// Key used to persist the phone number across page reloads (e.g. when the user
// refreshes the Cloudflare verification page).
const PHONE_NUMBER_STORAGE_KEY = "phoneNumber";

const readStoredPhoneNumber = (): string | undefined => {
  try {
    return sessionStorage.getItem(PHONE_NUMBER_STORAGE_KEY) ?? undefined;
  } catch {
    return undefined;
  }
};

// Provider component
export const AppProvider: React.FC<AppProviderProps> = ({ children }) => {
  const [phoneNumber, setPhoneNumberState] = useState<string | undefined>(
    readStoredPhoneNumber
  );

  // Persist the phone number so it survives a page reload. The phone number is
  // not sensitive enough to warrant special handling, and sessionStorage is
  // cleared when the tab is closed, scoping it to the current session.
  const setPhoneNumber = useCallback((value: string) => {
    setPhoneNumberState(value);
    try {
      if (value) {
        sessionStorage.setItem(PHONE_NUMBER_STORAGE_KEY, value);
      } else {
        sessionStorage.removeItem(PHONE_NUMBER_STORAGE_KEY);
      }
    } catch {
      // Ignore storage errors (e.g. private mode); state still works in-memory.
    }
  }, []);

  return (
    <AppContext.Provider value={{ phoneNumber, setPhoneNumber }}>
      {children}
    </AppContext.Provider>
  );
};

// Custom hook for consuming the context
export const useAppContext = (): AppContextType => {
  const context = useContext(AppContext);
  if (!context) {
    throw new Error("useAppContext must be used within an AppProvider");
  }
  return context;
};