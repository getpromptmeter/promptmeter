import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ChangeIndicator } from "@/components/shared/change-indicator";

interface KPICardProps {
  label: string;
  value: string;
  changePercent: number | null;
  invertColor?: boolean;
  neutral?: boolean;
  isLoading?: boolean;
}

export function KPICard({
  label,
  value,
  changePercent,
  invertColor,
  neutral,
  isLoading,
}: KPICardProps) {
  if (isLoading) {
    return (
      <Card>
        <CardContent className="p-4">
          <Skeleton className="h-4 w-24 mb-2" />
          <Skeleton className="h-8 w-32 mb-1" />
          <Skeleton className="h-4 w-16" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardContent className="p-4">
        <p className="text-sm font-medium text-muted-foreground">{label}</p>
        <p className="mt-1 text-2xl font-bold tracking-tight">{value}</p>
        <div className="mt-1">
          <ChangeIndicator
            value={changePercent}
            invertColor={invertColor}
            neutral={neutral}
          />
        </div>
      </CardContent>
    </Card>
  );
}
