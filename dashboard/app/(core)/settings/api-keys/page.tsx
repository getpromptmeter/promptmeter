"use client";

import { useState } from "react";
import { toast } from "sonner";
import { Plus } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { CopyButton } from "@/components/shared/copy-button";
import {
  useApiKeys,
  useCreateApiKey,
  useRevokeApiKey,
} from "@/lib/api/hooks/use-api-keys";
import { formatDistanceToNow } from "date-fns";

export default function APIKeysPage() {
  const { data: keys, isLoading } = useApiKeys();
  const createKey = useCreateApiKey();
  const revokeKey = useRevokeApiKey();
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [revokeDialogOpen, setRevokeDialogOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<{
    id: string;
    name: string;
    key_prefix: string;
  } | null>(null);
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  // Create form state
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyType, setNewKeyType] = useState<"live" | "test">("live");

  async function handleCreate() {
    try {
      const result = await createKey.mutateAsync({
        name: newKeyName,
        type: newKeyType,
      });
      setCreatedKey(result.key || "");
      setNewKeyName("");
      toast.success("API key created");
    } catch {
      toast.error("Failed to create API key");
    }
  }

  async function handleRevoke() {
    if (!selectedKey) return;
    try {
      await revokeKey.mutateAsync(selectedKey.id);
      setRevokeDialogOpen(false);
      setSelectedKey(null);
      toast.success("API key revoked");
    } catch {
      toast.error("Failed to revoke API key");
    }
  }

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>API Keys</CardTitle>
          <Button
            size="sm"
            onClick={() => {
              setCreatedKey(null);
              setCreateDialogOpen(true);
            }}
          >
            <Plus className="mr-1 h-4 w-4" />
            Create Key
          </Button>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-3">
              {[1, 2].map((i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : !keys || keys.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No API keys yet. Create one to get started.
            </p>
          ) : (
            <div className="space-y-2">
              <div className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 text-xs font-medium text-muted-foreground px-2">
                <span>Name</span>
                <span>Key</span>
                <span>Created</span>
                <span>Last Used</span>
                <span></span>
              </div>
              {keys.map((key) => (
                <div
                  key={key.id}
                  className="grid grid-cols-[1fr_auto_auto_auto_auto] gap-4 items-center rounded-md border px-3 py-2"
                >
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium">{key.name}</span>
                    {key.revoked_at && (
                      <Badge variant="destructive" className="text-xs">
                        Revoked
                      </Badge>
                    )}
                  </div>
                  <code className="text-xs text-muted-foreground font-mono">
                    {key.key_prefix}...
                  </code>
                  <span className="text-xs text-muted-foreground">
                    {formatDistanceToNow(new Date(key.created_at), {
                      addSuffix: true,
                    })}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {key.last_used_at
                      ? formatDistanceToNow(new Date(key.last_used_at), {
                          addSuffix: true,
                        })
                      : "Never"}
                  </span>
                  <div>
                    {!key.revoked_at && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="text-destructive"
                        onClick={() => {
                          setSelectedKey({
                            id: key.id,
                            name: key.name,
                            key_prefix: key.key_prefix,
                          });
                          setRevokeDialogOpen(true);
                        }}
                      >
                        Revoke
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Create Key Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent>
          {createdKey ? (
            <>
              <DialogHeader>
                <DialogTitle>API Key Created</DialogTitle>
                <DialogDescription>
                  This is the only time you will see the full key. Copy it now.
                </DialogDescription>
              </DialogHeader>
              <div className="rounded-md border bg-muted p-3 font-mono text-sm break-all">
                {createdKey}
                <CopyButton value={createdKey} className="mt-2" />
              </div>
              <DialogFooter>
                <Button onClick={() => setCreateDialogOpen(false)}>Done</Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <DialogHeader>
                <DialogTitle>Create API Key</DialogTitle>
              </DialogHeader>
              <div className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Name</label>
                  <Input
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    placeholder="Production SDK"
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Type</label>
                  <div className="flex gap-4">
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="radio"
                        checked={newKeyType === "live"}
                        onChange={() => setNewKeyType("live")}
                      />
                      Live (pm_live_*)
                    </label>
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="radio"
                        checked={newKeyType === "test"}
                        onChange={() => setNewKeyType("test")}
                      />
                      Test (pm_test_*)
                    </label>
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => setCreateDialogOpen(false)}
                >
                  Cancel
                </Button>
                <Button
                  onClick={handleCreate}
                  disabled={!newKeyName || createKey.isPending}
                >
                  {createKey.isPending ? "Creating..." : "Create"}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Revoke Confirmation Dialog */}
      <Dialog open={revokeDialogOpen} onOpenChange={setRevokeDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke API Key?</DialogTitle>
            <DialogDescription>
              This will immediately invalidate the key &quot;{selectedKey?.name}
              &quot; ({selectedKey?.key_prefix}). All requests using this key
              will fail.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setRevokeDialogOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRevoke}
              disabled={revokeKey.isPending}
            >
              {revokeKey.isPending ? "Revoking..." : "Revoke Key"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
