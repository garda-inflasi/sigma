"use client";

import React, { Component, type ReactNode } from "react";
import { AlertTriangle, RotateCw } from "lucide-react";
import { SpringButton } from "./spring-button";
import { cn } from "@/lib/utils";

interface Props {
  children: ReactNode;
  fallbackTitle?: string;
  className?: string;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class WidgetErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error("WidgetErrorBoundary caught an error:", error, errorInfo);
  }

  handleReset = () => {
    this.setState({ hasError: false, error: undefined });
  };

  render() {
    if (this.state.hasError) {
      return (
        <div
          className={cn(
            "glass flex min-h-[220px] flex-col items-center justify-center rounded-lg p-6 text-center",
            this.props.className
          )}
        >
          <div className="grid size-10 place-items-center rounded-lg bg-defisit-soft text-defisit-ink dark:bg-defisit/15 dark:text-rose-300">
            <AlertTriangle className="size-5" strokeWidth={1.75} aria-hidden />
          </div>
          <h4 className="mt-3 text-sm font-medium text-foreground">
            {this.props.fallbackTitle ?? "Gagal memuat bagian ini"}
          </h4>
          <p className="mt-1 max-w-sm text-[13px] text-muted-foreground">
            {this.state.error?.message ?? "Terjadi kendala saat memuat data tampilan."}
          </p>
          <SpringButton
            onClick={this.handleReset}
            size="sm"
            variant="soft"
            className="mt-4 gap-1.5"
          >
            <RotateCw className="size-3.5" />
            Coba lagi
          </SpringButton>
        </div>
      );
    }

    return this.props.children;
  }
}
