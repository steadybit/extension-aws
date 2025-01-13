// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extec2

const (
	azBlackholeActionId      = "com.steadybit.extension_aws.az.blackhole"
	azTargetType             = "com.steadybit.extension_aws.zone"
	azIcon                   = "data:image/svg+xml,%3Csvg%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%3E%0A%3Cpath%20fill-rule%3D%22evenodd%22%20clip-rule%3D%22evenodd%22%20d%3D%22M10.3743%204.03767C10.8996%203.931%2011.4432%203.875%2012%203.875C12.5567%203.875%2013.1004%203.931%2013.6257%204.03766C13.9882%204.64242%2014.3139%205.41721%2014.5808%206.32501H9.41913C9.68604%205.41721%2010.0117%204.64243%2010.3743%204.03767ZM14.9895%208.07501H9.01043C8.84181%209.01233%208.73009%2010.0377%208.69074%2011.125H15.3092C15.2699%2010.0377%2015.1582%209.01233%2014.9895%208.07501ZM17.0602%2011.125C17.0244%2010.065%2016.9238%209.03985%2016.7651%208.07501H19.1158C19.6254%208.99688%2019.961%2010.0283%2020.0784%2011.125H17.0602ZM15.3092%2012.875H8.69074C8.73009%2013.9623%208.84181%2014.9877%209.01044%2015.925H14.9895C15.1582%2014.9877%2015.2699%2013.9623%2015.3092%2012.875ZM16.7651%2015.925C16.9238%2014.9601%2017.0244%2013.935%2017.0602%2012.875H20.0784C19.961%2013.9717%2019.6254%2015.0031%2019.1158%2015.925H16.7651ZM14.5808%2017.675H9.41913C9.68605%2018.5828%2010.0117%2019.3576%2010.3743%2019.9623C10.8996%2020.069%2011.4433%2020.125%2012%2020.125C12.5567%2020.125%2013.1004%2020.069%2013.6257%2019.9623C13.9882%2019.3576%2014.3139%2018.5828%2014.5808%2017.675ZM15.9526%2019.1005C16.1173%2018.6534%2016.2657%2018.1766%2016.3966%2017.675H17.8147C17.268%2018.235%2016.6411%2018.7164%2015.9526%2019.1005ZM16.3966%206.32501C16.2657%205.82339%2016.1173%205.34665%2015.9526%204.89953C16.6411%205.28364%2017.268%205.76499%2017.8147%206.32501H16.3966ZM8.04739%204.89955C7.88268%205.34666%207.73424%205.82339%207.60333%206.32501H6.18535C6.73199%205.765%207.35886%205.28365%208.04739%204.89955ZM7.23487%208.07501H4.88421C4.37463%208.99688%204.03899%2010.0283%203.92157%2011.125H6.93973C6.97558%2010.065%207.07621%209.03985%207.23487%208.07501ZM7.23487%2015.925C7.07622%2014.9601%206.97559%2013.935%206.93973%2012.875H3.92157C4.03899%2013.9717%204.37463%2015.0031%204.88421%2015.925H7.23487ZM6.18535%2017.675H7.60333C7.73424%2018.1766%207.88268%2018.6533%208.04739%2019.1005C7.35887%2018.7163%206.73199%2018.235%206.18535%2017.675ZM12%202.125C6.54619%202.125%202.125%206.54619%202.125%2012C2.125%2017.4538%206.54619%2021.875%2012%2021.875C17.4538%2021.875%2021.875%2017.4538%2021.875%2012C21.875%206.54619%2017.4538%202.125%2012%202.125Z%22%20fill%3D%22%231D2632%22%2F%3E%0A%3C%2Fsvg%3E%0A"
	ec2InstanceStateActionId = "com.steadybit.extension_aws.ec2_instance.state"
	ec2TargetType            = "com.steadybit.extension_aws.ec2-instance"
	ec2Icon                  = "data:image/svg+xml,%3Csvg%20width%3D%2224%22%20height%3D%2224%22%20viewBox%3D%220%200%2024%2024%22%20fill%3D%22none%22%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%3E%0A%3Cpath%20fill-rule%3D%22evenodd%22%20clip-rule%3D%22evenodd%22%20d%3D%22M22.04%202.54998C21.83%202.33998%2021.56%202.22998%2021.27%202.22998H11.79C11.5%202.22998%2011.23%202.33998%2011.02%202.54998C10.81%202.75998%2010.7%203.02998%2010.7%203.31998V5.59998H12.09V3.61998H20.97V12.51H18.99V13.9H21.27C21.56%2013.9%2021.84%2013.78%2022.04%2013.58C22.25%2013.37%2022.36%2013.1%2022.36%2012.81V3.31998C22.36%203.02998%2022.25%202.74998%2022.04%202.54998ZM12.27%2021.2H3.39V12.32H5.37V10.93H3.09C2.8%2010.93%202.53%2011.04%202.32%2011.25C2.11%2011.46%202%2011.73%202%2012.02V21.5C2%2021.79%202.11%2022.06%202.32%2022.27C2.53%2022.48%202.8%2022.59%203.09%2022.59H12.57C12.86%2022.59%2013.13%2022.48%2013.34%2022.27C13.54%2022.07%2013.66%2021.79%2013.66%2021.5V19.22H12.27V21.2ZM16.83%207.02998C17%207.08998%2017.15%207.17998%2017.28%207.30998C17.41%207.43998%2017.5%207.58998%2017.56%207.75998H18.8V9.14998H17.61V9.73998H18.8V11.13H17.61V11.72H18.8V13.11H17.61V13.69H18.8V15.08H17.61V15.66H18.8V17.05H17.56C17.5%2017.22%2017.41%2017.37%2017.28%2017.5C17.15%2017.63%2017%2017.72%2016.83%2017.78V19.02H15.44V17.83H14.86V19.02H13.47V17.83H12.89V19.02H11.5V17.83H10.91V19.02H9.52001V17.83H8.93001V19.02H7.54001V17.78C7.37001%2017.72%207.22001%2017.62%207.09001%2017.5C6.96001%2017.38%206.87001%2017.22%206.81001%2017.05H5.57001V15.66H6.76001V15.08H5.57001V13.69H6.76001V13.11H5.57001V11.72H6.76001V11.13H5.57001V9.73998H6.76001V9.14998H5.57001V7.75998H6.81001C6.87001%207.58998%206.96001%207.43998%207.09001%207.30998C7.21001%207.17998%207.37001%207.08998%207.54001%207.02998V5.78998H8.93001V6.97998H9.52001V5.78998H10.91V6.97998H11.5V5.78998H12.89V6.97998H13.47V5.78998H14.86V6.97998H15.44V5.78998H16.83V7.02998ZM8.14001%2016.46H16.23V16.45V8.35998H8.14001V16.46Z%22%20fill%3D%22currentColor%22%2F%3E%0A%3C%2Fsvg%3E"
)
