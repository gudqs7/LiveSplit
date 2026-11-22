using LiveSplit.Model;
using LiveSplit.Options.SettingsFactories;
using LiveSplit.UI;
using LiveSplit.UI.Components;
using LiveSplit.View;
using System;
using System.Collections.Generic;
using System.Drawing;
using System.Linq;
using System.Runtime.ExceptionServices;
using System.Threading;
using System.Windows.Forms;
using System.Xml;
using Xunit;

namespace LiveSplit.Tests.UI;

public class LayoutSettingsDialogDpiMust
{
    [Fact]
    public void KeepComponentSettingsWithinTabWidthAfterDpiScaling()
    {
        RunInSta(() =>
        {
            var settings = new StandardLayoutSettingsFactory().Create();
            var layout = new Layout { Settings = settings };
            layout.LayoutComponents.Add(new LayoutComponent("Fake", new FakeFontComponent(new FakeSettingsControl())));

            using var dialog = new LayoutSettingsDialog(settings, layout)
            {
                ShowInTaskbar = false,
                StartPosition = FormStartPosition.Manual,
                Location = new Point(-32000, -32000)
            };
            using var scaledFont = new Font(dialog.Font.FontFamily, dialog.Font.Size * 1.5f);
            dialog.Font = scaledFont;

            try
            {
                dialog.Show();
                Application.DoEvents();

                TabControl tabControl = dialog.Controls.Find("tabControl", true)
                    .OfType<TabControl>()
                    .Single();
                TabPage page = tabControl.TabPages.Cast<TabPage>()
                    .Single(tabPage => tabPage.Name == "Fake Settings");

                tabControl.SelectedTab = page;
                Application.DoEvents();

                Assert.NotEmpty(page.Controls);
                foreach (Control child in page.Controls)
                {
                    Assert.True(
                        child.Right <= page.ClientSize.Width,
                        $"{child.GetType().Name} ends at {child.Right}px but the tab page is only {page.ClientSize.Width}px wide.");
                }

                int textHeight = Math.Max(
                    tabControl.Font.Height,
                    TextRenderer.MeasureText("Ag", tabControl.Font).Height);
                Assert.True(
                    tabControl.GetTabRect(0).Height >= textHeight + 6,
                    $"Tab header height {tabControl.GetTabRect(0).Height}px is too small for {textHeight}px text.");
            }
            finally
            {
                dialog.Close();
            }
        });
    }

    [Fact]
    public void KeepSumOfBestAccuracyGroupAtReadableHeightAfterDpiScaling()
    {
        RunInSta(() =>
        {
            var settings = new StandardLayoutSettingsFactory().Create();
            var layout = new Layout { Settings = settings };
            layout.LayoutComponents.Add(new LayoutComponent("SumOfBest", new FakeFontComponent(new SumOfBestSettings())));

            using var dialog = new LayoutSettingsDialog(settings, layout)
            {
                ShowInTaskbar = false,
                StartPosition = FormStartPosition.Manual,
                Location = new Point(-32000, -32000)
            };
            using var scaledFont = new Font(dialog.Font.FontFamily, dialog.Font.Size * 1.5f);
            dialog.Font = scaledFont;

            try
            {
                dialog.Show();
                Application.DoEvents();

                TabControl tabControl = dialog.Controls.Find("tabControl", true)
                    .OfType<TabControl>()
                    .Single();
                TabPage page = tabControl.TabPages.Cast<TabPage>()
                    .Single(tabPage => tabPage.Name == "Fake Settings");
                tabControl.SelectedTab = page;
                Application.DoEvents();

                GroupBox accuracyGroup = page.Controls.Find("groupBox4", true)
                    .OfType<GroupBox>()
                    .Single();
                TableLayoutPanel accuracyLayout = accuracyGroup.Controls.Find("tableLayoutPanel5", true)
                    .OfType<TableLayoutPanel>()
                    .Single();
                RadioButton firstOption = accuracyLayout.Controls.OfType<RadioButton>().First();

                Assert.True(
                    accuracyLayout.ClientSize.Height >= firstOption.PreferredSize.Height + 2,
                    $"Accuracy options are {firstOption.PreferredSize.Height}px high but only {accuracyLayout.ClientSize.Height}px is available.");
                Assert.True(
                    accuracyGroup.ClientSize.Height > accuracyLayout.Bottom,
                    $"Accuracy layout ends at {accuracyLayout.Bottom}px inside a {accuracyGroup.ClientSize.Height}px group box.");
            }
            finally
            {
                dialog.Close();
            }
        });
    }

    private static void RunInSta(Action action)
    {
        Exception exception = null;
        var thread = new Thread(() =>
        {
            try
            {
                action();
            }
            catch (Exception ex)
            {
                exception = ex;
            }
        });
        thread.SetApartmentState(ApartmentState.STA);
        thread.Start();
        thread.Join();

        if (exception != null)
        {
            ExceptionDispatchInfo.Capture(exception).Throw();
        }
    }

    private sealed class FakeSettingsControl : UserControl
    {
        public FakeSettingsControl()
        {
            AutoScaleDimensions = new SizeF(6F, 13F);
            AutoScaleMode = AutoScaleMode.Font;
            Size = new Size(459, 600);
        }
    }

    [GlobalFontConsumer(GlobalFont.TimerFont)]
    private sealed class FakeFontComponent : IComponent
    {
        private readonly Control settingsControl;

        public FakeFontComponent(Control settingsControl)
        {
            this.settingsControl = settingsControl;
        }

        public string ComponentName => "Fake Settings";
        public float HorizontalWidth => 200;
        public float MinimumHeight => 20;
        public float VerticalHeight => 100;
        public float MinimumWidth => 20;
        public float PaddingTop => 0;
        public float PaddingBottom => 0;
        public float PaddingLeft => 0;
        public float PaddingRight => 0;
        public IDictionary<string, Action> ContextMenuControls => null;

        public void DrawHorizontal(Graphics g, LiveSplitState state, float height, Region clipRegion)
        {
        }

        public void DrawVertical(Graphics g, LiveSplitState state, float width, Region clipRegion)
        {
        }

        public Control GetSettingsControl(LayoutMode mode) => settingsControl;

        public XmlNode GetSettings(XmlDocument document) => document.CreateElement("Settings");

        public void SetSettings(XmlNode settings)
        {
        }

        public void Update(IInvalidator invalidator, LiveSplitState state, float width, float height, LayoutMode mode)
        {
        }

        public void Dispose()
        {
        }
    }
}
