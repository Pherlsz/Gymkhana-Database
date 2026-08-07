export type CatalogV1 = {
  auth: {
    login: {
      title: string;
      subtitle: string;
      logoAlt: string;
      googleButton: string;
      developmentButton: string;
      googleStartError: string;
      developmentStartError: string;
    };
  };
  shell: {
    productName: string;
    navigationLabel: string;
    navigation: {
      home: string;
      profiles: string;
      search: string;
      query: string;
      tasks: string;
      matching: string;
      chat: string;
      ocr: string;
      customData: string;
      operations: string;
      googleForms: string;
    };
    roles: {
      external: string;
      admin: string;
      superadmin: string;
    };
    signOut: string;
    signingOut: string;
  };
  home: {
    eyebrow: string;
    description: string;
    sessionActive: string;
    userAdministrationTitle: string;
    userAdministrationDescription: string;
    foundationTitle: string;
    foundationDescription: string;
  };
};
