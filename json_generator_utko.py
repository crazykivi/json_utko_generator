import pandas as pd
import json
import os

def safe_division(numerator, denominator):
    """Безопасное деление с проверкой на ноль и NaN"""
    try:
        if pd.isna(numerator) or pd.isna(denominator) or float(denominator) == 0:
            return 0
        return round(float(numerator) / float(denominator), 2)
    except:
        return 0

def excel_to_json(file_path):
    df = pd.read_excel(file_path, header=None, skiprows=2)
    df.columns = ['A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X',
                  'Y', 'Z', 'AA', 'AB', 'AC', 'AD', 'AE', 'AF', 'AG']
    
    object_type_map = {
        'размещение': 'DisposalObject',
        'обработка': 'AssortmentObject',
        'обезвреживание': 'IncinerationObject',
        'утилизация': 'CompostingObject',
        'хранение': 'StorageObject',
        'перегрузка': 'TransshipmentObject'
    }

    result = {"Routes": []}

    # Максимум вычисление до 3 плеча включительно
    for index, row in df.iterrows():
        try:
            # Вычисление TravelTime с защитой от ошибок
            travel_time_1 = safe_division(row['M'], row['O'])
            travel_time_2 = safe_division(row['V'], row['X'])
            travel_time_3 = safe_division(row['AE'], row['AG'])
            
            # Проверка, есть ли данные в третьем плече
            has_third_leg_data = (
                pd.notna(row['Y']) and   # ObjectId
                pd.notna(row['Z']) and   # ObjectType
                pd.notna(row['AA']) and  # Power
                pd.notna(row['AB']) and  # Volume
                pd.notna(row['AC']) and  # Latitude
                pd.notna(row['AD']) and  # Longitude
                pd.notna(row['AE']) and  # Distance
                pd.notna(row['AF']) and  # Tkm
                pd.notna(row['AG'])      # Speed
            )
            
            # Третье плечо
            route_3 = None
            if has_third_leg_data:
                route_3 = {
                    "PartNumber": 3,
                    "ObjectId": str(row['Y']),
                    "ObjectName": "3 Плечо",
                    # "ObjectType": "DisposalObject" if row['Z'] == 'размещение' else "AssortmentObject",
                    "ObjectType": object_type_map.get(row['Z'], 'UnknownObject'),
                    "Power": float(row['AA']),
                    "Volume": float(row['AB']),
                    "Latitude": float(row['AC']),
                    "Longitude": float(row['AD']),
                    "Polyline": None,
                    "Distance": float(row['AE']),
                    "Tkm": float(row['AF']),
                    "TravelTime": travel_time_3,
                    "Speed": float(row['AG']),
                    "Availability": 1,
                    "TransportType": 1
                }

            # Проверка, есть ли данные во втором плече
            has_second_leg_data = (
                pd.notna(row['P']) and  # ObjectId
                pd.notna(row['Q']) and  # ObjectType
                pd.notna(row['R']) and  # Power
                pd.notna(row['S']) and  # Volume
                pd.notna(row['T']) and  # Latitude
                pd.notna(row['U']) and  # Longitude
                pd.notna(row['V']) and  # Distance
                pd.notna(row['W']) and  # Tkm
                pd.notna(row['X'])      # Speed
            )

            # Второе плечо
            route_2 = None
            if has_second_leg_data:
                route_2 = {
                    "PartNumber": 2,
                    "ObjectId": str(row['P']),
                    "ObjectName": "2 Плечо",
                    #"ObjectType": "DisposalObject" if row['Q'] == 'размещение' else "AssortmentObject",
                    "ObjectType": object_type_map.get(row['Q'], 'UnknownObject'),
                    "Power": float(row['R']),
                    "Volume": float(row['S']),
                    "Latitude": float(row['T']),
                    "Longitude": float(row['U']),
                    "Polyline": None,
                    "Distance": float(row['V']),
                    "Tkm": float(row['W']),
                    "TravelTime": travel_time_2,
                    "Speed": float(row['X']),
                    "Availability": 1,
                    "TransportType": 1
                }
                # Привязка route_3 к route_2
                if route_3 is not None:
                    route_2["Route"] = route_3

            # Первое плечо
            route_1 = {
                "PartNumber": 1,
                "ObjectId": str(row['G']) if pd.notna(row['G']) else "",
                "ObjectName": "1 Плечо",
                #"ObjectType": "DisposalObject" if row['H'] == 'размещение' else "AssortmentObject",
                "ObjectType": object_type_map.get(row['H'], 'UnknownObject'),
                "Power": float(row['I']) if pd.notna(row['I']) else 0,
                "Volume": float(row['J']) if pd.notna(row['J']) else 0,
                "Latitude": float(row['K']) if pd.notna(row['K']) else 0,
                "Longitude": float(row['L']) if pd.notna(row['L']) else 0,
                "Polyline": None,
                "Distance": float(row['M']) if pd.notna(row['M']) else 0,
                "Tkm": float(row['N']) if pd.notna(row['N']) else 0,
                "TravelTime": travel_time_1,
                "Speed": float(row['O']) if pd.notna(row['O']) else 0,
                "Availability": 1,
                "TransportType": 1
            }

            # Второе плечо, только если он существует
            if route_2 is not None:
                route_1["Route"] = route_2

            # Population
            population = {
                "PopulationId": str(row['A']) if pd.notna(row['A']) else "",
                "PopulationName": str(row['C']) if pd.notna(row['B']) else "",
                "CountWaste": float(row['F']) if pd.notna(row['F']) else 0,
                "Latitude": float(row['D']) if pd.notna(row['D']) else 0,
                "Longitude": float(row['E']) if pd.notna(row['E']) else 0,
                "Route": route_1
            }

            result["Routes"].append(population)

        except Exception as e:
            print(f"Ошибка в строке {index}: {e}")
            continue

    return json.dumps(result, ensure_ascii=False, indent=2)

if __name__ == "__main__":
    folder_path = os.getcwd()
    excel_files = [f for f in os.listdir(folder_path) if f.endswith('.xlsx')]

    if not excel_files:
        print("В текущей папке нет Excel-файлов (.xlsx)")
        exit()

    print("Найденные Excel-файлы:")
    for i, file in enumerate(excel_files, start=1):
        print(f"{i}. {file}")

    choice = input("Введите номер файла для обработки: ")
    try:
        choice_idx = int(choice) - 1
        if not (0 <= choice_idx < len(excel_files)):
            raise ValueError
    except:
        print("Неверный выбор.")
        exit()

    input_file = excel_files[choice_idx]
    default_output = os.path.splitext(input_file)[0] + ".json"

    output_file = input(f"Введите имя для JSON-файла (Enter — по умолчанию '{default_output}'): ").strip()
    if not output_file:
        output_file = default_output
    elif not output_file.endswith('.json'):
        output_file += '.json'

    print(f"\nНачинаю обработку файла '{input_file}'...")
    json_output = excel_to_json(input_file)

    with open(output_file, "w", encoding="utf-8") as f:
        f.write(json_output)

    print(f"JSON успешно сохранён в '{output_file}'")