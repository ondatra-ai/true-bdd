import Rail from "../../components/Rail";
import DockedPanel from "../../components/DockedPanel";
import ChatDialog from "../../components/ChatDialog";
import { FilesProvider } from "../../components/FilesStore";

export default function WorkspaceLayout({ children }) {
  return (
    <FilesProvider>
      <div className="app-frame" data-testid="app-frame">
        <Rail />
        <DockedPanel />
        <div className="app-main">
          <div className="app-main__content">{children}</div>
          <ChatDialog />
        </div>
      </div>
    </FilesProvider>
  );
}
