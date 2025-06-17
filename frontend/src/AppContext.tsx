import React, { createContext, useContext, useState, ReactNode } from "react";

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

// Provider component
export const AppProvider: React.FC<AppProviderProps> = ({ children }) => {
  const [phoneNumber, setPhoneNumber] = useState<string | undefined>();

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