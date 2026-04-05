"use client";

import { useEffect } from "react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import { toast } from "sonner";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useSettings, useUpdateSettings } from "@/lib/api/hooks/use-settings";

const settingsSchema = z.object({
  name: z.string().min(1).max(100),
  timezone: z.string(),
  pii_enabled: z.boolean(),
  slack_webhook_url: z.string().optional(),
});

type SettingsForm = z.infer<typeof settingsSchema>;

export default function GeneralSettingsPage() {
  const { data: settings, isLoading } = useSettings();
  const updateSettings = useUpdateSettings();

  const form = useForm<SettingsForm>({
    defaultValues: {
      name: "",
      timezone: "UTC",
      pii_enabled: true,
      slack_webhook_url: "",
    },
  });

  useEffect(() => {
    if (settings) {
      form.reset({
        name: settings.name,
        timezone: settings.timezone,
        pii_enabled: settings.pii_enabled,
        slack_webhook_url: settings.slack_webhook_url || "",
      });
    }
  }, [settings, form]);

  async function onSubmit(data: SettingsForm) {
    try {
      await updateSettings.mutateAsync({
        name: data.name,
        timezone: data.timezone,
        pii_enabled: data.pii_enabled,
        slack_webhook_url: data.slack_webhook_url || undefined,
      });
      toast.success("Settings saved successfully");
    } catch {
      toast.error("Failed to save settings");
    }
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Organization Settings</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Organization Settings</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
          <div className="space-y-2">
            <label className="text-sm font-medium">Organization Name</label>
            <Input {...form.register("name")} />
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Timezone</label>
            <Input
              {...form.register("timezone")}
              placeholder="UTC"
            />
            <p className="text-xs text-muted-foreground">
              IANA timezone (e.g., America/New_York, Europe/London)
            </p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Data Privacy</label>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                {...form.register("pii_enabled")}
                className="rounded border-input"
              />
              <span className="text-sm">Store prompt and response text</span>
            </div>
            <p className="text-xs text-muted-foreground">
              When disabled, only metadata (tokens, cost, latency) is stored.
              Prompt/response text is never sent to the server.
            </p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium">Notifications</label>
            <label className="text-xs text-muted-foreground block">
              Slack Webhook URL
            </label>
            <Input
              {...form.register("slack_webhook_url")}
              placeholder="https://hooks.slack.com/services/..."
            />
          </div>

          <div className="flex justify-end">
            <Button type="submit" disabled={updateSettings.isPending}>
              {updateSettings.isPending ? "Saving..." : "Save Changes"}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
